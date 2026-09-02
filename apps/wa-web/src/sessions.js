// Baileys' export shape isn't stable across major versions: 6.x was CJS
// with the whole module bound to the default import, so pulling named
// exports off of `baileysPkg.default`/`baileysPkg` worked there. 7.x ships
// as real ESM with actual named exports and an empty default — importing
// it the old way silently gives back an empty object (no error at import
// time; makeWASocket etc. just come back `undefined`, which only surfaces
// later as "useMultiFileAuthState is not a function"). Named imports work
// correctly on both.
import { makeWASocket, useMultiFileAuthState, DisconnectReason, fetchLatestBaileysVersion } from '@whiskeysockets/baileys';
import QRCode from 'qrcode';
import { rmSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const AUTH_DIR = process.env.AUTH_DIR || path.join(__dirname, '..', 'auth');

// Per-chat message cap for backfill — keeps a single very long-running chat
// from ballooning one HTTP payload, not a limit on how many chats import.
// Every chat WhatsApp hands us in history sync gets imported (see
// _concludeHistoryImport below) — there is deliberately no chat-count cap
// anymore; a studio's real chat list shouldn't be truncated to an arbitrary
// top-N.
const BACKFILL_PER_CHAT_LIMIT = 200;

// Overall safety net — WhatsApp relays history through the user's primary
// phone, so it only arrives once that phone is online, and it can take
// anywhere from seconds to a couple of minutes for a real account with real
// history. This timer only fires to conclude the import if NOT EVEN ONE
// history-sync chunk ever arrives within a generous window (e.g. the
// primary phone stays offline), so the UI isn't stuck spinning forever.
const HISTORY_SYNC_TIMEOUT_MS = 3 * 60_000;

// Baileys' `isLatest` flag on a 'messaging-history.set' chunk is NOT a
// reliable "the whole sync is done" signal — confirmed against real traffic:
// WhatsApp can send a trivial early chunk (e.g. syncType INITIAL_STATUS_V3,
// zero chats/messages) already marked isLatest:true, with the real chat
// history (INITIAL_BOOTSTRAP / RECENT chunks, hundreds of messages) still to
// follow marked isLatest:false. Trusting isLatest here silently concluded
// the import — and dropped every subsequent real chunk — before any
// history had actually arrived. Instead, treat the sync as "settled" once
// this many ms pass with no new chunk arriving at all; reset on every chunk.
const HISTORY_SETTLE_MS = 5_000;

export class SessionManager {
  constructor({ log, projectxApiUrl }) {
    this.log = log;
    this.projectxApiUrl = projectxApiUrl;
    // Map<studioId, { sock, qr, status, phone, historyBuffer: Map<jid, msg[]> }>
    this.sessions = new Map();
    // Map<studioId, Promise<void>> — an in-flight _startSession call, if any.
    // _startSession awaits reading auth state and fetching the protocol
    // version BEFORE it registers the session in `this.sessions`, so two
    // callers that both check "is there already a session?" close together
    // (e.g. the startup prewarm loop and an admin clicking "Show QR Code" at
    // the same moment) can both pass that check and each open a real socket
    // against the same linked-device credentials. WhatsApp allows only one
    // live connection per device, so the second one causes the server to
    // boot the first with a conflict error (statusCode 440), and the two
    // fight over the connection until one gives up. Routing every entry
    // point through _ensureSession closes that window — concurrent callers
    // share the same in-flight start instead of racing to create another.
    this.starting = new Map();
  }

  async _ensureSession(studioId, opts) {
    if (this.sessions.has(studioId)) return;
    const inFlight = this.starting.get(studioId);
    if (inFlight) return inFlight;
    const p = this._startSession(studioId, opts).finally(() => this.starting.delete(studioId));
    this.starting.set(studioId, p);
    return p;
  }

  // Returns { status: 'connected'|'qr'|'pending'|'none', qr?, phone? }
  async getOrStartSession(studioId) {
    const existing = this.sessions.get(studioId);
    if (existing) {
      if (existing.status === 'connected') return { status: 'connected', phone: existing.phone };
      if (existing.status === 'qr' && existing.qr) return { status: 'qr', qr: existing.qr };
      return { status: 'pending' };
    }
    await this._ensureSession(studioId);
    // Wait up to 30s for the socket to connect and a QR to appear.
    for (let i = 0; i < 150; i++) {
      await sleep(200);
      const s = this.sessions.get(studioId);
      if (s?.status === 'qr' && s?.qr) return { status: 'qr', qr: s.qr };
      if (s?.status === 'connected') return { status: 'connected', phone: s.phone };
    }
    return { status: 'pending' };
  }

  getStatus(studioId) {
    const s = this.sessions.get(studioId);
    if (!s) return { status: 'none' };
    return { status: s.status, phone: s.phone || null };
  }

  // Diagnostic: checks whether a number is actually registered on WhatsApp
  // at all. sock.sendMessage() will still happily assign a message ID and
  // report success even when sending to a number with no WhatsApp account —
  // that check isn't automatic — so a message can look "sent" in our system
  // and never arrive anywhere, with nothing in our own logs to explain why.
  async checkOnWhatsApp(studioId, number) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const digits = number.replace(/[^\d]/g, '');
    const results = await s.sock.onWhatsApp(digits);
    return results;
  }

  // An explicit admin-initiated disconnect wipes the persisted auth session
  // too — not just the in-memory socket. Without this, the local auth/
  // folder survived a disconnect, so a later reconnect (even for a
  // different WhatsApp number entirely, since it's keyed by studioId, not
  // by phone number) would try to resume the OLD session's credentials
  // first. WhatsApp only pushes the one-time full history sync on a
  // genuinely fresh pairing (see isFreshPairing in _startSession) — leaving
  // stale creds around meant every reconnect after a manual disconnect
  // silently downgraded to a resume attempt instead of a real fresh link,
  // which is exactly the "0 imported" symptom this fixes.
  async disconnect(studioId) {
    const s = this.sessions.get(studioId);
    if (s?.historyTimer) clearTimeout(s.historyTimer);
    if (s?.historySettleTimer) clearTimeout(s.historySettleTimer);
    if (s?.sock) {
      // Mark this session as intentionally torn down *before* logging out —
      // sock.logout() itself fires a 'close' event on this same entry, and
      // the connection.update handler below unconditionally schedules an
      // auto-reconnect on close. Without this flag, an explicit disconnect
      // silently kicks off a brand-new session a second later (racing the
      // rmSync below against that new session's own auth-dir writes, which
      // can crash the whole process — see the 'close' handler).
      s.intentionalDisconnect = true;
      await s.sock.logout().catch(() => {});
      s.sock.end(undefined);
    }
    this.sessions.delete(studioId);
    rmSync(path.join(AUTH_DIR, studioId), { recursive: true, force: true });
  }

  // Throws if `to` has no WhatsApp account. sock.sendMessage() alone would
  // silently "succeed" against such a number (see checkOnWhatsApp above), so
  // every real send goes through this first. Skipped for values that are
  // already a resolved JID (@c.us / @lid / @s.whatsapp.net / @g.us) rather
  // than a raw phone number — those came from an established conversation
  // (message history, a live contact), so they're already known-good; @lid
  // in particular isn't a phone number at all, so running it through
  // onWhatsApp() would give a meaningless, likely false-negative result.
  // This matters most for brand-new numbers we've never messaged before —
  // e.g. a lead just imported from an external sheet or a Stripe checkout —
  // which is exactly the case that was silently failing.
  async assertOnWhatsApp(s, to) {
    if (/@(c\.us|lid|s\.whatsapp\.net|g\.us)$/.test(to)) return;
    const digits = to.replace(/[^\d]/g, '');
    const results = await s.sock.onWhatsApp(digits);
    const exists = Array.isArray(results) && results.some((r) => r?.exists);
    if (!exists) {
      throw new Error(`number ${digits} is not registered on WhatsApp`);
    }
  }

  async sendMessage(studioId, to, text) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    await this.assertOnWhatsApp(s, to);
    const jid = toJid(to);
    return s.sock.sendMessage(jid, { text });
  }

  async sendMedia(studioId, to, mediaUrl, mediaType, caption) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    await this.assertOnWhatsApp(s, to);
    const jid = toJid(to);
    const url = mediaUrl.startsWith('http') ? mediaUrl : `${this.projectxApiUrl}${mediaUrl}`;
    const kind = (mediaType || '').toLowerCase();
    const content = kind.startsWith('image')
      ? { image: { url }, caption }
      : kind.startsWith('video')
      ? { video: { url }, caption }
      : kind.startsWith('audio')
      ? { audio: { url } }
      : { document: { url }, caption, mimetype: 'application/octet-stream' };
    return s.sock.sendMessage(jid, content);
  }

  // Pre-warm a session for a studio so a QR is ready before the user clicks
  // "connect" — much cheaper now (a WebSocket, not a Chrome launch).
  async prewarm(studioId) {
    if (this.sessions.has(studioId) || this.starting.has(studioId)) return;
    this.log.info({ studioId }, 'wa-web: pre-warming session');
    await this._ensureSession(studioId);
  }

  // History import runs automatically in the background as soon as a session
  // connects (see _startSession / _concludeHistoryImport) — Baileys pushes
  // chat history on its own schedule, so there's no "right moment" for a
  // human to click a button and catch it. This is the manual entry point the
  // admin's "Import chat history" / "Retry" button hits; it never starts a
  // second, competing import. It just makes sure the Go API's view of the
  // current state is fresh:
  //   - already concluded  -> re-report the result (covers a wa-web restart
  //     losing its 'done' notification before the Go side received it)
  //   - still in flight     -> re-assert 'running' (covers the same restart
  //     scenario for the initial notification) and let it keep going
  //   - resumed session ('inert') -> no-op. WhatsApp already had its one
  //     chance to push history right after the original pairing and won't
  //     repeat it on a plain reconnect, so there's nothing to (re-)run —
  //     leave whatever Go already has on record untouched.
  async backfillHistory(studioId) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);

    if (s.historyState === 'done') {
      await this._notifyWithRetry('/internal/wa-web/backfill-done', {
        studioId,
        messageCount: s.historyImportedCount ?? 0,
        failed: !!s.historyImportFailed,
      });
      return;
    }
    if (s.historyState === 'inert') {
      this.log.info({ studioId }, 'wa-web: history import requested on a resumed session — nothing to do');
      return;
    }
    await this._notifyBackfillRunning(studioId);
  }

  // Called once per session, either when the history-sync chunk stream goes
  // quiet for HISTORY_SETTLE_MS (see the messaging-history.set handler below —
  // NOT Baileys' own `isLatest` flag, which turned out unreliable, see
  // HISTORY_SETTLE_MS's comment) or when the overall safety-net timer in
  // _startSession fires because not even one chunk ever arrived. Idempotent —
  // whichever fires first wins, the other is a no-op.
  async _concludeHistoryImport(studioId) {
    const entry = this.sessions.get(studioId);
    if (!entry || entry.historyState === 'done') return;
    entry.historyState = 'done';
    if (entry.historyTimer) {
      clearTimeout(entry.historyTimer);
      entry.historyTimer = null;
    }
    if (entry.historySettleTimer) {
      clearTimeout(entry.historySettleTimer);
      entry.historySettleTimer = null;
    }

    let totalImported = 0;
    let failed = false;
    try {
      const chatEntries = [...entry.historyBuffer.entries()];
      this.log.info(
        {
          studioId,
          chatCount: chatEntries.length,
          bufferedMessages: chatEntries.reduce((n, [, msgs]) => n + msgs.length, 0),
          perChat: chatEntries.map(([jid, msgs]) => ({ jid, count: msgs.length, hasName: !!entry.historyDisplayNames.get(jid) })),
        },
        'wa-web: history import starting',
      );
      for (const [jid, messages] of chatEntries) {
        if (messages.length === 0) continue;
        try {
          const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/backfill`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              studioId,
              messages: messages.slice(0, BACKFILL_PER_CHAT_LIMIT),
              displayName: entry.historyDisplayNames.get(jid) || undefined,
            }),
          });
          if (res.ok) {
            const body = await res.json().catch(() => ({}));
            totalImported += body.imported || 0;
          } else {
            this.log.warn({ studioId, status: res.status }, 'wa-web: history import chat push non-ok');
          }
        } catch (err) {
          this.log.warn({ err: err.message, studioId }, 'wa-web: history import chat failed, continuing');
        }
      }

      // Names for contacts WhatsApp told us about but that never got a
      // message bucket above (a saved contact with no chat at all, or a
      // chat whose only messages were non-text and got skipped) would
      // otherwise be silently dropped — push those separately so they still
      // show a real name instead of a bare phone number.
      for (const [rawJid, name] of entry.historyDisplayNames.entries()) {
        if (entry.historyBuffer.has(rawJid)) continue; // already carried a name above
        const jid = await resolveJid(entry.sock, rawJid, entry.lidToPhone, this.log);
        if (!jid || entry.historyBuffer.has(jid)) continue;
        await this._pushContactName(studioId, jid, name);
      }
    } catch (err) {
      this.log.error({ err: { message: err.message, stack: err.stack }, studioId }, 'wa-web: history import failed');
      failed = true;
    }

    entry.historyImportedCount = totalImported;
    entry.historyImportFailed = failed;
    await this._notifyWithRetry('/internal/wa-web/backfill-done', { studioId, messageCount: totalImported, failed });
    this.log.info({ studioId, totalImported, failed }, 'wa-web: history import finished');
  }

  async _notifyBackfillRunning(studioId) {
    await this._notifyWithRetry('/internal/wa-web/backfill-running', { studioId });
  }

  // Pushes a single contact's display name to the Go API — used both by the
  // end-of-backfill sweep above and by the live contacts.upsert/update
  // listeners below, so a name learned at any point (during or long after
  // the initial history sync) reaches an existing conversation instead of
  // only ever being applied once at connect time.
  async _pushContactName(studioId, jid, name) {
    try {
      const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/contact-name`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId, jid, displayName: name }),
      });
      if (!res.ok) {
        this.log.warn({ studioId, jid, status: res.status }, 'wa-web: contact-name push non-ok');
      }
    } catch (err) {
      this.log.warn({ err: err.message, studioId, jid }, 'wa-web: contact-name push failed, continuing');
    }
  }

  // Restarts the "conclude the import" countdown — called on every
  // history-sync chunk (see messaging-history.set), including empty ones,
  // so the import only concludes once the chunk stream actually goes quiet.
  _resetHistorySettleTimer(studioId) {
    const entry = this.sessions.get(studioId);
    if (!entry || entry.historyState !== 'pending') return;
    if (entry.historySettleTimer) clearTimeout(entry.historySettleTimer);
    entry.historySettleTimer = setTimeout(() => {
      this._concludeHistoryImport(studioId).catch((err) =>
        this.log.error({ err, studioId }, 'wa-web: history import failed'),
      );
    }, HISTORY_SETTLE_MS);
  }

  async _startSession(studioId, { reconnectAttempts = 0 } = {}) {
    const dataPath = path.join(AUTH_DIR, studioId);
    const { state, saveCreds } = await useMultiFileAuthState(dataPath);
    // The WA Web version baked into a given @whiskeysockets/baileys release
    // goes stale as WhatsApp ships updates — a stale version gets the
    // handshake rejected with a 405 "Connection Failure" right after
    // "not logged in, attempting registration...", before a QR is ever
    // emitted. Ask WhatsApp's version endpoint for the current one instead
    // of trusting the library default.
    const { version } = await fetchLatestBaileysVersion().catch(() => ({ version: undefined }));

    // WhatsApp only ever pushes the post-pairing history dump once, right
    // after a device is freshly linked — not on every plain reconnect of an
    // already-linked session. Gate the whole history-import lifecycle on
    // that so a reconnect can't spuriously reset an already-completed import
    // back to "running", or report a fake "done, 0 messages" that would
    // overwrite a real earlier count.
    const isFreshPairing = !state.creds.registered;

    const entry = {
      status: 'pending',
      qr: null,
      phone: null,
      sock: null,
      historyBuffer: new Map(),
      // jid -> WhatsApp display name, pulled from the same history-sync
      // payload as the messages themselves (see messaging-history.set below).
      // Used to populate contact_identities.display_name during backfill —
      // deliberately NOT used to create a lead (see handleWAWebBackfillOne /
      // HandleInboundWAWebBackfill on the Go side, which already keep
      // history import from spawning phantom leads on purpose).
      historyDisplayNames: new Map(),
      // 'pending' while listening for Baileys' one-time post-pairing history
      // push; 'done' once concluded (see _concludeHistoryImport); 'inert' for
      // a resumed (non-fresh) session, which never gets one, so there's
      // nothing to wait for or report.
      historyState: isFreshPairing ? 'pending' : 'inert',
      historyTimer: null,
      // Debounce timer reset on every history-sync chunk (see
      // messaging-history.set below) — concludes the import once the chunk
      // stream goes quiet for HISTORY_SETTLE_MS, rather than trusting
      // Baileys' own (unreliable) isLatest flag.
      historySettleTimer: null,
      historyImportedCount: undefined,
      historyImportFailed: undefined,
      // @lid (WhatsApp's internal linked-device ID) -> real @c.us phone JID.
      // Baileys' history sync doesn't expose this mapping (see
      // 'chats.phoneNumberShare' listener below for how it's actually
      // populated), so a @lid chat only gets a display phone number once
      // WhatsApp shares it — not immediately for every contact.
      lidToPhone: new Map(),
      reconnectAttempts,
    };
    this.sessions.set(studioId, entry);

    const sock = makeWASocket({
      auth: state,
      version,
      // Without this Baileys only syncs app-state (contacts, minimal recent
      // context) after pairing — 'messaging-history.set' never fires with
      // real chat messages, so backfillHistory() always imports 0.
      syncFullHistory: true,
      // Baileys 7.x's OWN default for this rejects syncType FULL outright
      // (`shouldSyncHistoryMessage: ({ syncType }) => syncType !== FULL`,
      // see its Defaults/index.js) — and FULL is exactly the sync chunk
      // that carries real prior chat history; RECENT/INITIAL_BOOTSTRAP
      // chunks alone can come back with zero messages. Confirmed via
      // production logs: a FULL notification logged "process: false" and
      // never reached our messaging-history.set handler at all, which is
      // why history import kept reporting 0 even on a fresh pairing.
      shouldSyncHistoryMessage: () => true,
      logger: this.log.child({ studioId, component: 'baileys' }),
      printQRInTerminal: false,
    });
    entry.sock = sock;

    sock.ev.on('creds.update', saveCreds);

    sock.ev.on('connection.update', async (update) => {
      const { connection, qr, lastDisconnect } = update;

      if (qr) {
        try {
          entry.qr = await QRCode.toDataURL(qr, { width: 300, margin: 2 });
          entry.status = 'qr';
          this.log.info({ studioId }, 'wa-web: qr generated');
        } catch (err) {
          this.log.error({ err }, 'wa-web: qrcode generation failed');
        }
        return;
      }

      if (connection === 'open') {
        const phone = (sock.user?.id || '').split(':')[0].split('@')[0];
        entry.status = 'connected';
        entry.qr = null;
        entry.phone = phone;
        entry.reconnectAttempts = 0;
        this.log.info({ studioId, phone }, 'wa-web: connected');
        await this._notifyConnected(studioId, phone);

        if (entry.historyState === 'pending') {
          await this._notifyBackfillRunning(studioId);
          entry.historyTimer = setTimeout(() => {
            this._concludeHistoryImport(studioId).catch((err) =>
              this.log.error({ err, studioId }, 'wa-web: history import (timeout path) failed'),
            );
          }, HISTORY_SYNC_TIMEOUT_MS);
        }
        return;
      }

      if (connection === 'close') {
        const statusCode = lastDisconnect?.error?.output?.statusCode;
        const loggedOut = statusCode === DisconnectReason.loggedOut;
        this.log.warn({ studioId, statusCode, loggedOut }, 'wa-web: disconnected');

        if (entry.historyTimer) {
          clearTimeout(entry.historyTimer);
          entry.historyTimer = null;
        }
        entry.status = 'pending';
        entry.qr = null;
        this.sessions.delete(studioId);
        await this._notifyDisconnected(studioId);

        if (loggedOut) {
          // Session was explicitly logged out (e.g. unlinked from the phone) —
          // the stored auth is no longer valid, so clear it and wait for a
          // fresh QR scan rather than looping on a dead credential set.
          rmSync(dataPath, { recursive: true, force: true });
        }
        if (entry.intentionalDisconnect) return;

        // Reconnect with exponential backoff (capped at 30s) — a tight,
        // unthrottled reconnect loop on a genuine protocol/handshake failure
        // just hammers WhatsApp's servers repeatedly instead of recovering.
        const attempts = (entry.reconnectAttempts || 0) + 1;
        const delayMs = Math.min(30_000, 1000 * 2 ** (attempts - 1));
        this.log.info({ studioId, attempts, delayMs }, 'wa-web: reconnecting after backoff');
        setTimeout(() => {
          this._ensureSession(studioId, { reconnectAttempts: attempts }).catch((err) =>
            this.log.error({ err, studioId }, 'wa-web: reconnect after disconnect failed'),
          );
        }, delayMs);
      }
    });

    sock.ev.on('messages.upsert', async ({ messages, type }) => {
      if (type !== 'notify') return;
      for (const msg of messages) {
        // Group messages carry the group's @g.us JID as remoteJid — the same
        // one for every message in that group regardless of which member
        // sent it — so a group comes through as one conversation thread
        // (not split per-member), same as the messaging-history.set
        // (backfill) handler below.

        // fromMe messages are forwarded too, not discarded — the Go side
        // dedupes these against messages our own outbound worker already
        // sent (matched by WhatsApp message ID, see waWebSender.SendText).
        // A fromMe message that ISN'T one of ours is one the studio typed
        // directly into WhatsApp on the linked phone, and needs recording
        // just as much as a customer's reply does.
        const text = extractMessageText(msg.message);
        this.log.info(
          { studioId, from: msg.key?.remoteJid, fromMe: !!msg.key?.fromMe, body: text.slice(0, 50) },
          'wa-web: message received',
        );
        if (!text) continue;
        await this._forwardInbound(studioId, {
          from: await resolveJid(sock, msg.key.remoteJid, entry.lidToPhone, this.log),
          text,
          messageId: msg.key.id,
          timestamp: Number(msg.messageTimestamp) || Math.floor(Date.now() / 1000),
          fromMe: !!msg.key?.fromMe,
          // WhatsApp attaches the sender's own display name to every message
          // they send — pushName — so a brand-new contact gets a real name
          // immediately, without waiting on history sync to (maybe) supply
          // one later. IS present on our own fromMe messages too (confirmed
          // via logs) — but there it's the studio's OWN account name, not
          // the customer's, and must never be used as their display name
          // (this got a real contact's identity permanently locked to the
          // studio's own name on their first-ever, studio-sent message).
          pushName: !msg.key?.fromMe ? (msg.pushName || undefined) : undefined,
        });
      }
    });

    // Buffer Baileys' automatic post-pairing history push. Fires in one or
    // more chunks (`syncType`-dependent) shortly after a fresh QR link. Does
    // NOT trust `isLatest` to mean "sync is fully done" — see
    // HISTORY_SETTLE_MS's comment for why that flag is unreliable here.
    // Instead, every chunk (whether or not it carries anything importable)
    // resets a settle timer at the bottom of this handler; the import is
    // concluded once that timer fires with no new chunk having arrived.
    sock.ev.on('messaging-history.set', async ({ chats, contacts, messages, isLatest, syncType, progress }) => {
      // Always log the raw chunk, even when nothing ends up qualifying below —
      // otherwise a chunk that arrives but yields zero importable messages
      // (all group chats, all non-text content, etc.) looks identical in the
      // logs to the sync never having fired at all, and that distinction
      // matters for diagnosing "why did nothing import".
      this.log.info(
        { studioId, syncType, isLatest, progress, chatsInChunk: chats?.length ?? 0, messagesInChunk: messages?.length ?? 0 },
        'wa-web: history-sync chunk received',
      );

      if (entry.historyState !== 'pending') return; // resumed session — nothing to capture

      // Same chunk carries each chat's WhatsApp display name (chats[].name /
      // .displayName, contacts[].name) alongside the messages — capture it so
      // the imported conversation shows a real name instead of a bare phone
      // number. Keyed by the raw (pre-resolution) id for now; re-keyed under
      // the resolved jid below once we know it, since chats/contacts here can
      // reference a contact by either its @lid or phone form.
      let namesInChunk = 0;
      for (const c of chats || []) {
        const name = c.displayName || c.name;
        if (name && c.id) { entry.historyDisplayNames.set(c.id, name); namesInChunk++; }
      }
      for (const c of contacts || []) {
        if (c.name && c.id) { entry.historyDisplayNames.set(c.id, c.name); namesInChunk++; }
      }
      // Diagnostic: dump exactly what WhatsApp handed us for chats/contacts —
      // id + name/displayName as-is, before any jid resolution — so a
      // "why didn't the name show up" report can be checked against the raw
      // wire data instead of guessing about jid-format mismatches blind.
      if ((chats?.length ?? 0) > 0 || (contacts?.length ?? 0) > 0) {
        this.log.info(
          {
            studioId,
            namesInChunk,
            totalNamesKnown: entry.historyDisplayNames.size,
            chatsSample: (chats || []).slice(0, 10).map((c) => ({ id: c.id, name: c.name, displayName: c.displayName })),
            contactsSample: (contacts || []).slice(0, 10).map((c) => ({ id: c.id, name: c.name })),
          },
          'wa-web: history-sync chunk — chat/contact names seen',
        );
      }

      if (!messages?.length) {
        this._resetHistorySettleTimer(studioId);
        return;
      }
      let skippedNoJid = 0;
      let skippedNoText = 0;
      let namedMessages = 0;
      let unnamedMessages = 0;
      const unmatchedSample = [];
      for (const msg of messages) {
        // fromMe (a message the studio itself sent) is imported too, same as
        // live messages (see handleWAWebBackfillOne on the Go side, which
        // already records it as outbound/SourceStudioUser) — a chat whose
        // history is mostly or entirely outbound must not come out as "no
        // history to import" just because the customer's replies happen to
        // be sparse or absent.
        const rawJid = msg.key.remoteJid;
        const jid = await resolveJid(sock, rawJid, entry.lidToPhone, this.log);
        if (!jid) { skippedNoJid++; continue; } // no resolvable jid at all
        const text = extractMessageText(msg.message);
        if (!text) { skippedNoText++; continue; }
        // chats[]/contacts[] essentially never carry a name for a 1:1 chat
        // (confirmed by direct log inspection — WhatsApp's linked-device
        // protocol doesn't sync the phone's private address book to a web
        // session at all, only group subjects and a couple of system
        // entries). msg.pushName — the sender's own self-declared WhatsApp
        // display name, broadcast on every message they send — is the only
        // other name source available, and IS present on historical messages
        // same as live ones. Not the phone's saved contact name, but the
        // closest thing actually obtainable here; only trust it for
        // messages actually from the contact (fromMe's pushName is the
        // studio's own name, not theirs).
        const name = entry.historyDisplayNames.get(rawJid) || entry.historyDisplayNames.get(jid)
          || (!msg.key.fromMe ? msg.pushName : undefined);
        if (name) {
          entry.historyDisplayNames.set(jid, name); // ensure it's reachable under the resolved key too
          namedMessages++;
        } else {
          unnamedMessages++;
          if (unmatchedSample.length < 10) unmatchedSample.push({ rawJid, jid, fromMe: !!msg.key.fromMe, pushName: msg.pushName });
        }
        const bucket = entry.historyBuffer.get(jid) || [];
        bucket.push({
          from: jid,
          text,
          messageId: msg.key.id,
          timestamp: Number(msg.messageTimestamp) || Math.floor(Date.now() / 1000),
          fromMe: !!msg.key.fromMe,
        });
        entry.historyBuffer.set(jid, bucket);
      }
      // Diagnostic: for every message that DIDN'T resolve to a known name,
      // show exactly what jid we looked it up under vs what's actually in
      // historyDisplayNames — pinpoints a jid-format mismatch (e.g. chats[]
      // keyed by @lid while messages arrive keyed by phone JID) instead of
      // guessing about it.
      if (namedMessages || unnamedMessages) {
        this.log.info(
          { studioId, namedMessages, unnamedMessages, unmatchedSample, knownNameKeys: [...entry.historyDisplayNames.keys()].slice(0, 20) },
          'wa-web: history-sync chunk — name match results',
        );
      }
      if (skippedNoJid || skippedNoText) {
        this.log.info({ studioId, skippedNoJid, skippedNoText }, 'wa-web: history-sync chunk — messages skipped');
      }
      this._resetHistorySettleTimer(studioId);
    });

    // Fired when WhatsApp explicitly shares the real phone number behind a
    // @lid chat (a SHARE_PHONE_NUMBER protocol message) — the only source
    // Baileys exposes for @lid -> phone-number resolution. Not guaranteed
    // for every contact, so @lid chats without one just display the LID.
    sock.ev.on('chats.phoneNumberShare', ({ lid, jid }) => {
      if (!lid || !jid) return;
      entry.lidToPhone.set(lid, normalizeJid(jid, entry.lidToPhone));
      this.log.info({ studioId, lid, jid }, 'wa-web: resolved @lid to phone number');
    });

    // The actual source of a contact's saved name — confirmed by direct log
    // inspection that messaging-history.set's own chats[]/contacts[] arrays
    // carry a `name` for group chats and little else; a 1:1 chat's saved
    // address-book name (what shows in the phone's own WhatsApp app) comes
    // through WhatsApp's separate contact app-state sync instead, which
    // Baileys surfaces as these three events — 'contacts.set' fires once
    // with the full list on a fresh pairing's initial sync (parallel to
    // 'messaging-history.set' for messages), 'contacts.upsert'/'.update'
    // fire incrementally afterward. `c.name` is the phone's saved contact
    // name (set on this device); `c.notify` is the fallback — the pushName
    // the other party set on their own account. Fires independently of
    // message history sync and can arrive before, during, or well after it,
    // so every name here is both cached (for backfill/live message-name
    // lookups still in flight) and pushed immediately (in case backfill has
    // already concluded and a conversation is already sitting there with no
    // name).
    const onContacts = async (contacts) => {
      for (const c of contacts || []) {
        const name = c.name || c.notify;
        if (!name || !c.id) continue;
        // Never record the studio's own account as if it were a contact —
        // WhatsApp's contact sync includes a self-entry, and its "name" is
        // the studio's own WhatsApp profile name (e.g. the studio's brand),
        // not a customer's. Matched by stripped numeric prefix so it still
        // catches device-suffixed / @lid forms of the same own-number.
        const numericId = c.id.replace(/^(\d+).*$/, '$1');
        if (entry.phone && numericId === entry.phone.replace(/^\+/, '')) continue;
        entry.historyDisplayNames.set(c.id, name);
        const jid = await resolveJid(sock, c.id, entry.lidToPhone, this.log);
        if (jid) {
          entry.historyDisplayNames.set(jid, name);
          await this._pushContactName(studioId, jid, name);
        }
      }
    };
    sock.ev.on('contacts.set', ({ contacts }) => onContacts(contacts));
    sock.ev.on('contacts.upsert', onContacts);
    sock.ev.on('contacts.update', onContacts);
  }

  // POSTs with a few retries (backing off) so a transient Go-API restart
  // right when a session connects/disconnects doesn't leave channel_accounts
  // permanently out of sync with the real session state. Each attempt is
  // logged; a final failure is logged as an error but never throws — the
  // periodic reconciliation loop (startStatusReconciliation) is the backstop
  // that eventually corrects the DB even if every retry here fails.
  async _notifyWithRetry(path, body, { attempts = 4 } = {}) {
    for (let i = 1; i <= attempts; i++) {
      try {
        const res = await fetch(`${this.projectxApiUrl}${path}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        if (res.ok) return true;
        this.log.warn({ path, body, status: res.status, attempt: i }, 'wa-web: notify non-ok');
      } catch (err) {
        this.log.warn({ err: err.message, path, body, attempt: i }, 'wa-web: notify failed, retrying');
      }
      if (i < attempts) await sleep(i * 1000);
    }
    this.log.error({ path, body }, 'wa-web: notify failed after all retries — will self-heal on next status reconciliation');
    return false;
  }

  async _notifyConnected(studioId, phone) {
    await this._notifyWithRetry('/internal/wa-web/connected', { studioId, phone });
  }

  async _notifyDisconnected(studioId) {
    await this._notifyWithRetry('/internal/wa-web/disconnected', { studioId });
  }

  // Periodic backstop: re-pushes the real in-memory session status for every
  // known session to the Go API, so channel_accounts.status can never drift
  // permanently out of sync even if a connect/disconnect notification was
  // missed entirely (e.g. Go API was down through all retry attempts above).
  startStatusReconciliation(intervalMs = 60_000) {
    setInterval(() => {
      for (const [studioId, s] of this.sessions.entries()) {
        if (s.status === 'connected' && s.phone) {
          this._notifyWithRetry('/internal/wa-web/connected', { studioId, phone: s.phone }, { attempts: 1 })
            .catch(() => {});
        }
      }
    }, intervalMs);
  }

  async _forwardInbound(studioId, { from, text, messageId, timestamp, fromMe, pushName }) {
    try {
      const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/inbound`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId, from, text, messageId, timestamp, fromMe: !!fromMe, pushName }),
      });
      const body = await res.text();
      this.log.info({ studioId, from, status: res.status, body }, 'wa-web: forwarded inbound');
    } catch (err) {
      this.log.error({ err, studioId }, 'wa-web: forward inbound failed');
    }
  }
}

// Pulls the plain-text body out of a Baileys message object. Plain-text
// messages carry it directly (`conversation` / `extendedTextMessage`), but
// disappearing messages and view-once messages wrap the real content one
// level deeper — read literally, those wrappers have no text of their own,
// so a message can be 100% real text and still come back empty without
// unwrapping them first. Recurses since these wrappers can nest (e.g. a
// view-once message sent with disappearing messages also on).
function extractMessageText(message) {
  if (!message) return '';
  if (message.conversation) return message.conversation;
  if (message.extendedTextMessage?.text) return message.extendedTextMessage.text;
  const wrapped =
    message.ephemeralMessage?.message ||
    message.viewOnceMessage?.message ||
    message.viewOnceMessageV2?.message ||
    message.viewOnceMessageV2Extension?.message ||
    message.documentWithCaptionMessage?.message;
  return wrapped ? extractMessageText(wrapped) : '';
}

// WhatsApp JIDs can carry a ":<deviceId>" suffix on the user portion for
// multi-device addressing (e.g. "917483974512:0@s.whatsapp.net" — device 0
// of that number). We identify contacts by phone number, not by which of
// their devices sent a given message, so this must be stripped before the
// JID is used as a conversation/contact key — otherwise the same person
// messaging from two devices (or even just Baileys echoing back a fromMe
// send with the device-qualified form) would fragment into multiple
// contacts, or display as "917483974512:0" instead of a clean number.
function stripDeviceSuffix(jid) {
  return jid.replace(/^(\d+):\d+@/, '$1@');
}

// Normalizes a Baileys JID to the @c.us / @lid convention the Go API's
// identity-stitching logic already expects (it previously only ever saw
// whatsapp-web.js's addressing scheme). Cache-only — does not attempt a
// fresh lookup. Used where the jid in hand is already a real phone JID
// (e.g. the target of a chats.phoneNumberShare event) or where an async
// lookup genuinely isn't possible.
function normalizeJid(jid, lidToPhone) {
  if (!jid) return jid;
  jid = stripDeviceSuffix(jid);
  if (jid.endsWith('@s.whatsapp.net')) return jid.replace('@s.whatsapp.net', '@c.us');
  if (jid.endsWith('@lid') && lidToPhone?.has(jid)) return lidToPhone.get(jid);
  return jid; // unresolved @lid and @g.us pass through as-is
}

// Actively resolves a @lid JID to the real phone number behind it, using
// Baileys' LIDMappingStore (sock.signalRepository.lidMapping) — added in
// Baileys 7.x specifically to make this possible. Older 6.x had no such
// API; the only signal was a passive chats.phoneNumberShare event that
// WhatsApp fires at its own discretion, which left most @lid contacts
// permanently showing as a raw numeric ID instead of a phone number.
// Falls back to the cache-only behavior (and ultimately the raw jid) if
// the lookup throws or comes back empty — WhatsApp doesn't guarantee a
// mapping is available for every contact.
async function resolveJid(sock, jid, lidToPhone, log) {
  if (!jid) return jid;
  jid = stripDeviceSuffix(jid);
  if (jid.endsWith('@s.whatsapp.net')) return jid.replace('@s.whatsapp.net', '@c.us');
  if (!jid.endsWith('@lid')) return jid; // @g.us passes through as-is
  if (lidToPhone.has(jid)) return lidToPhone.get(jid);
  try {
    const pn = await sock.signalRepository?.lidMapping?.getPNForLID(jid);
    if (pn) {
      const resolved = stripDeviceSuffix(pn.endsWith('@s.whatsapp.net') ? pn.replace('@s.whatsapp.net', '@c.us') : pn);
      lidToPhone.set(jid, resolved);
      log?.info({ lid: jid, resolved }, 'wa-web: resolved @lid to phone number via lidMapping store');
      return resolved;
    }
  } catch (err) {
    log?.warn({ err: err.message, lid: jid }, 'wa-web: lidMapping lookup failed, leaving unresolved');
  }
  return jid; // no mapping available yet — display the raw lid until one is
}

// Converts a stored contact value (bare digits, or already-suffixed
// @c.us/@lid from the Go side) into a Baileys-addressable JID.
function toJid(to) {
  if (to.endsWith('@c.us')) return to.replace('@c.us', '@s.whatsapp.net');
  if (to.endsWith('@lid') || to.endsWith('@s.whatsapp.net') || to.endsWith('@g.us')) return to;
  const digits = to.replace(/[^\d]/g, '');
  return `${digits}@s.whatsapp.net`;
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}
