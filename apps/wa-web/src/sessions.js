// Baileys is a CJS package; its default-export interop shape has changed
// across versions (the default import bound to the whole module.exports
// object rather than the makeWASocket function on at least one version we
// hit in practice). Pull it defensively off the namespace import instead of
// trusting `import makeWASocket from '...'` to resolve correctly.
import baileysPkg from '@whiskeysockets/baileys';
const makeWASocket = baileysPkg.makeWASocket || baileysPkg.default;
const { useMultiFileAuthState, DisconnectReason, fetchLatestBaileysVersion } = baileysPkg;
import QRCode from 'qrcode';
import { rmSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const AUTH_DIR = process.env.AUTH_DIR || path.join(__dirname, '..', 'auth');

// Baileys history-sync buffer bounds — mirrors the old whatsapp-web.js
// backfill defaults (maxChats / perChatLimit) so downstream behavior
// (Go API payload sizes, admin UX expectations) doesn't change.
const BACKFILL_MAX_CHATS = 50;
const BACKFILL_PER_CHAT_LIMIT = 200;

// Safety net only — WhatsApp relays history through the user's primary phone,
// so it only arrives once that phone is online and it can take anywhere from
// seconds to a couple of minutes for a real account with real history. We
// don't guess when it's "probably not coming"; we wait for Baileys' own
// `isLatest` completion signal on 'messaging-history.set' (see
// _startSession). This timer only fires to conclude the import if that
// signal never arrives at all within a generous window (e.g. the primary
// phone stays offline), so the UI isn't stuck spinning forever.
const HISTORY_SYNC_TIMEOUT_MS = 3 * 60_000;

export class SessionManager {
  constructor({ log, projectxApiUrl }) {
    this.log = log;
    this.projectxApiUrl = projectxApiUrl;
    // Map<studioId, { sock, qr, status, phone, historyBuffer: Map<jid, msg[]> }>
    this.sessions = new Map();
  }

  // Returns { status: 'connected'|'qr'|'pending'|'none', qr?, phone? }
  async getOrStartSession(studioId) {
    const existing = this.sessions.get(studioId);
    if (existing) {
      if (existing.status === 'connected') return { status: 'connected', phone: existing.phone };
      if (existing.status === 'qr' && existing.qr) return { status: 'qr', qr: existing.qr };
      return { status: 'pending' };
    }
    await this._startSession(studioId);
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

  async disconnect(studioId) {
    const s = this.sessions.get(studioId);
    if (s?.historyTimer) clearTimeout(s.historyTimer);
    if (s?.sock) {
      await s.sock.logout().catch(() => {});
      s.sock.end(undefined);
    }
    this.sessions.delete(studioId);
  }

  async sendMessage(studioId, to, text) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const jid = toJid(to);
    return s.sock.sendMessage(jid, { text });
  }

  async sendMedia(studioId, to, mediaUrl, mediaType, caption) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
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
    if (this.sessions.has(studioId)) return;
    this.log.info({ studioId }, 'wa-web: pre-warming session');
    await this._startSession(studioId);
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

  // Called once per session, either when Baileys signals the post-pairing
  // history sync is complete (`isLatest` on 'messaging-history.set') or when
  // the safety-net timer in _startSession fires because that signal never
  // came. Idempotent — whichever fires first wins, the other is a no-op.
  async _concludeHistoryImport(studioId) {
    const entry = this.sessions.get(studioId);
    if (!entry || entry.historyState === 'done') return;
    entry.historyState = 'done';
    if (entry.historyTimer) {
      clearTimeout(entry.historyTimer);
      entry.historyTimer = null;
    }

    let totalImported = 0;
    let failed = false;
    try {
      const chatEntries = [...entry.historyBuffer.entries()].slice(0, BACKFILL_MAX_CHATS);
      this.log.info(
        { studioId, chatCount: entry.historyBuffer.size, processing: chatEntries.length },
        'wa-web: history import starting',
      );
      for (const [, messages] of chatEntries) {
        if (messages.length === 0) continue;
        try {
          const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/backfill`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ studioId, messages: messages.slice(0, BACKFILL_PER_CHAT_LIMIT) }),
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
      // 'pending' while listening for Baileys' one-time post-pairing history
      // push; 'done' once concluded (see _concludeHistoryImport); 'inert' for
      // a resumed (non-fresh) session, which never gets one, so there's
      // nothing to wait for or report.
      historyState: isFreshPairing ? 'pending' : 'inert',
      historyTimer: null,
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
        // Reconnect with exponential backoff (capped at 30s) — a tight,
        // unthrottled reconnect loop on a genuine protocol/handshake failure
        // just hammers WhatsApp's servers repeatedly instead of recovering.
        const attempts = (entry.reconnectAttempts || 0) + 1;
        const delayMs = Math.min(30_000, 1000 * 2 ** (attempts - 1));
        this.log.info({ studioId, attempts, delayMs }, 'wa-web: reconnecting after backoff');
        setTimeout(() => {
          this._startSession(studioId, { reconnectAttempts: attempts }).catch((err) =>
            this.log.error({ err, studioId }, 'wa-web: reconnect after disconnect failed'),
          );
        }, delayMs);
      }
    });

    sock.ev.on('messages.upsert', async ({ messages, type }) => {
      if (type !== 'notify') return;
      for (const msg of messages) {
        if (msg.key?.fromMe) continue;
        const text = extractMessageText(msg.message);
        this.log.info({ studioId, from: msg.key?.remoteJid, body: text.slice(0, 50) }, 'wa-web: message received');
        if (!text) continue;
        await this._forwardInbound(studioId, {
          from: normalizeJid(msg.key.remoteJid, entry.lidToPhone),
          text,
          messageId: msg.key.id,
          timestamp: Number(msg.messageTimestamp) || Math.floor(Date.now() / 1000),
        });
      }
    });

    // Buffer Baileys' automatic post-pairing history push. Fires in one or
    // more chunks (`syncType`-dependent) shortly after a fresh QR link;
    // `isLatest` marks the final chunk, which is what actually concludes the
    // import (see _concludeHistoryImport) — buffer size alone can't tell a
    // completed sync from one still mid-flight.
    sock.ev.on('messaging-history.set', ({ chats, messages, isLatest, syncType, progress }) => {
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

      if (!messages?.length) {
        if (isLatest) {
          this._concludeHistoryImport(studioId).catch((err) =>
            this.log.error({ err, studioId }, 'wa-web: history import failed'),
          );
        }
        return;
      }
      let skippedGroup = 0;
      let skippedNoText = 0;
      for (const msg of messages) {
        if (msg.key?.fromMe) continue;
        const jid = normalizeJid(msg.key.remoteJid, entry.lidToPhone);
        if (!jid || jid.endsWith('@g.us')) { skippedGroup++; continue; } // skip group chats, matches old behavior
        const text = extractMessageText(msg.message);
        if (!text) { skippedNoText++; continue; }
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
      if (skippedGroup || skippedNoText) {
        this.log.info({ studioId, skippedGroup, skippedNoText }, 'wa-web: history-sync chunk — messages skipped');
      }
      if (isLatest) {
        this._concludeHistoryImport(studioId).catch((err) =>
          this.log.error({ err, studioId }, 'wa-web: history import failed'),
        );
      }
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

  async _forwardInbound(studioId, { from, text, messageId, timestamp }) {
    try {
      const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/inbound`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId, from, text, messageId, timestamp }),
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

// Normalizes a Baileys JID to the @c.us / @lid convention the Go API's
// identity-stitching logic already expects (it previously only ever saw
// whatsapp-web.js's addressing scheme).
function normalizeJid(jid, lidToPhone) {
  if (!jid) return jid;
  if (jid.endsWith('@s.whatsapp.net')) return jid.replace('@s.whatsapp.net', '@c.us');
  if (jid.endsWith('@lid') && lidToPhone?.has(jid)) return lidToPhone.get(jid);
  return jid; // unresolved @lid and @g.us pass through as-is
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
