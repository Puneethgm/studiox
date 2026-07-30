// Telegram's MTProto QR login differs from WhatsApp's pairing flow (see
// apps/wa-web/src/sessions.js for that one): instead of one QR string that's
// valid until scanned, Telegram issues a short-lived login token and expects
// a fresh one to be requested on expiry. teleproto's signInUserWithQrCode
// already loops internally, calling our `qrCode` callback again each time it
// mints a new token — we just re-render whatever token we're handed most
// recently and keep waiting.
import { TelegramClient, sessions as tgSessions, events, client as tgClient, Api } from 'teleproto';
import QRCode from 'qrcode';

const { StringSession } = tgSessions;
const { NewMessage } = events;
const { CustomFile } = tgClient.uploads;

const BACKFILL_MAX_CHATS = 50;
const BACKFILL_PER_CHAT_LIMIT = 200;

export class SessionManager {
  constructor({ log, projectxApiUrl, apiId, apiHash }) {
    this.log = log;
    this.projectxApiUrl = projectxApiUrl;
    this.apiId = apiId;
    this.apiHash = apiHash;
    // Map<studioId, { client, status, qr, phone, username, sessionString,
    //                  historyState, resolvePassword, passwordHint, abortController }>
    this.sessions = new Map();
    // Map<studioId, Promise<void>> — in-flight login, mirrors wa-web's
    // `starting` map so concurrent QR requests share one attempt instead of
    // racing to start two logins for the same studio.
    this.starting = new Map();
    // Map<studioId, sessionString> — persisted sessions loaded from the Go
    // API at startup so a tg-web restart can resume without a fresh QR scan.
    // Populated by loadPersistedSession(); a session only ever gets HERE
    // after a successful login (see _persistSession).
    this.persisted = new Map();
  }

  async _ensureSession(studioId, persistedSessionString) {
    if (this.sessions.has(studioId)) return;
    const inFlight = this.starting.get(studioId);
    if (inFlight) return inFlight;
    const p = this._startSession(studioId, persistedSessionString).finally(() => this.starting.delete(studioId));
    this.starting.set(studioId, p);
    return p;
  }

  // Returns { status: 'connected'|'qr'|'password_required'|'pending'|'error', qr?, phone?, username?, passwordHint?, error? }
  async getOrStartSession(studioId) {
    const existing = this.sessions.get(studioId);
    if (existing) return this._publicStatus(existing);
    try {
      await this._ensureSession(studioId, this.persisted.get(studioId));
    } catch (err) {
      // _startSession already recorded {status:'error', error} on the entry
      // before rethrowing (see connect()/signInUserWithQrCode catch blocks)
      // — report that instead of letting this reject the whole request, so
      // a failed login attempt surfaces as a normal 200 + error message the
      // frontend can render, not a 500 the polling loop just retries blindly.
      const failed = this.sessions.get(studioId);
      if (failed) return this._publicStatus(failed);
      return { status: 'error', error: err.message };
    }
    for (let i = 0; i < 150; i++) {
      await sleep(200);
      const s = this.sessions.get(studioId);
      if (!s) continue;
      if (s.status === 'qr' && s.qr) return this._publicStatus(s);
      if (s.status === 'connected') return this._publicStatus(s);
      if (s.status === 'password_required') return this._publicStatus(s);
      if (s.status === 'error') return this._publicStatus(s);
    }
    return { status: 'pending' };
  }

  _publicStatus(s) {
    if (s.status === 'connected') return { status: 'connected', phone: s.phone, username: s.username };
    if (s.status === 'qr') return { status: 'qr', qr: s.qr };
    if (s.status === 'password_required') return { status: 'password_required', passwordHint: s.passwordHint || null };
    if (s.status === 'error') return { status: 'error', error: s.error };
    return { status: 'pending' };
  }

  getStatus(studioId) {
    const s = this.sessions.get(studioId);
    if (!s) return { status: 'none' };
    return this._publicStatus(s);
  }

  // Submits a 2FA password for a session waiting on one (see the `password`
  // callback in _startSession). No-op if this session isn't actually
  // waiting — the caller learns nothing was pending from the response.
  async submitPassword(studioId, password) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'password_required' || !s.resolvePassword) {
      return { accepted: false };
    }
    s.resolvePassword(password);
    s.resolvePassword = null;
    return { accepted: true };
  }

  async disconnect(studioId) {
    const s = this.sessions.get(studioId);
    if (s?.client) {
      try {
        // Best-effort real logout (invalidates the session on Telegram's
        // side) — falls back to a plain disconnect if that RPC fails so a
        // flaky network doesn't block the local session from being torn down.
        await s.client.invoke(new Api.auth.LogOut()).catch(() => {});
        await s.client.destroy();
      } catch (err) {
        this.log.warn({ err: err.message, studioId }, 'tg-web: disconnect error (continuing)');
      }
    }
    this.sessions.delete(studioId);
    this.persisted.delete(studioId);
  }

  async sendMessage(studioId, to, text) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const entity = await resolveEntity(s.client, to);
    return s.client.sendMessage(entity, { message: text });
  }

  // Downloads the attachment's bytes ourselves rather than handing
  // GramJS a bare URL for it to fetch — a relative /uploads/... path
  // resolves against projectxApiUrl (the internal Docker/localhost
  // address tg-web already talks to the Go API on), which is exactly
  // right for a fetch originating from tg-web itself, but is NOT reachable
  // from Telegram's own servers if we instead handed them that URL
  // directly (same class of problem the webhook URL has — see
  // docs/SETUP_TELEGRAM_QR.md). Downloading here and passing the actual
  // bytes as `file` sidesteps the "must be public" requirement entirely.
  async sendMedia(studioId, to, mediaUrl, mediaType, caption) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const entity = await resolveEntity(s.client, to);

    const isLocal = !mediaUrl.startsWith('http');
    const fetchURL = isLocal ? `${this.projectxApiUrl}${mediaUrl}` : mediaUrl;
    const res = await fetch(fetchURL);
    if (!res.ok) throw new Error(`fetch attachment failed: HTTP ${res.status}`);
    const buffer = Buffer.from(await res.arrayBuffer());
    const file = new CustomFile(
      mediaUrl.split('/').pop() || 'file',
      buffer.length,
      '',
      buffer,
    );

    return s.client.sendMessage(entity, { message: caption || '', file });
  }

  // Pre-warm: resumes a persisted session (if any) so it's already
  // 'connected' before an admin opens the Channels page. Does NOT start a
  // fresh QR login — that only happens when the admin explicitly requests
  // one via getOrStartSession, since minting a login token unprompted has
  // no one to scan it.
  async prewarm(studioId) {
    const sessionString = this.persisted.get(studioId);
    if (!sessionString || this.sessions.has(studioId) || this.starting.has(studioId)) return;
    this.log.info({ studioId }, 'tg-web: pre-warming session from persisted credentials');
    await this._ensureSession(studioId, sessionString);
  }

  registerPersistedSession(studioId, sessionString) {
    this.persisted.set(studioId, sessionString);
  }

  async backfillHistory(studioId) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    if (s.historyState === 'done') {
      await this._notifyWithRetry('/internal/tg-web/backfill-done', {
        studioId,
        messageCount: s.historyImportedCount ?? 0,
        failed: !!s.historyImportFailed,
      });
      return;
    }
    if (s.historyState === 'running') return; // already in flight
    this._runHistoryImport(studioId).catch((err) =>
      this.log.error({ err: err.message, studioId }, 'tg-web: history import failed'),
    );
  }

  async _runHistoryImport(studioId) {
    const s = this.sessions.get(studioId);
    if (!s) return;
    s.historyState = 'running';
    await this._notifyWithRetry('/internal/tg-web/backfill-running', { studioId });

    let totalImported = 0;
    let failed = false;
    try {
      const dialogs = await s.client.getDialogs({ limit: BACKFILL_MAX_CHATS });
      const directDialogs = dialogs.filter((d) => d.isUser && !d.entity?.bot);
      this.log.info({ studioId, dialogCount: dialogs.length, direct: directDialogs.length }, 'tg-web: history import starting');

      for (const dialog of directDialogs) {
        try {
          const messages = await s.client.getMessages(dialog.entity, { limit: BACKFILL_PER_CHAT_LIMIT });
          const payload = messages
            .filter((m) => m.text)
            .map((m) => ({
              chatId: dialog.id.toString(),
              text: m.text,
              messageId: String(m.id),
              timestamp: m.date,
              fromMe: !!m.out,
            }));
          if (payload.length === 0) continue;

          const res = await fetch(`${this.projectxApiUrl}/internal/tg-web/backfill`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ studioId, messages: payload, displayName: dialog.name || dialog.title }),
          });
          if (res.ok) {
            const body = await res.json().catch(() => ({}));
            totalImported += body.imported || 0;
          } else {
            this.log.warn({ studioId, status: res.status }, 'tg-web: history import chat push non-ok');
          }
        } catch (err) {
          this.log.warn({ err: err.message, studioId }, 'tg-web: history import chat failed, continuing');
        }
      }
    } catch (err) {
      this.log.error({ err: err.message, studioId }, 'tg-web: history import failed');
      failed = true;
    }

    s.historyState = 'done';
    s.historyImportedCount = totalImported;
    s.historyImportFailed = failed;
    await this._notifyWithRetry('/internal/tg-web/backfill-done', { studioId, messageCount: totalImported, failed });
    this.log.info({ studioId, totalImported, failed }, 'tg-web: history import finished');
  }

  async _startSession(studioId, persistedSessionString) {
    const stringSession = new StringSession(persistedSessionString || '');
    const client = new TelegramClient(stringSession, this.apiId, this.apiHash, {
      connectionRetries: 5,
    });

    const entry = {
      status: 'pending',
      qr: null,
      phone: null,
      username: null,
      client,
      historyState: 'pending',
      historyImportedCount: undefined,
      historyImportFailed: undefined,
      resolvePassword: null,
      passwordHint: null,
      error: null,
    };
    this.sessions.set(studioId, entry);

    // The MTProto handshake itself (raw TCP to a Telegram datacenter IP,
    // not plain HTTPS) is the step most likely to silently hang — e.g. an
    // outbound firewall that only allows standard web ports/hosts. Without
    // a timeout here, a stuck connect() leaves the session parked on
    // 'pending' forever with nothing in the logs to explain why, and every
    // poll from the frontend just gets a content-free 200. Time-box it and
    // surface a real error instead.
    this.log.info({ studioId }, 'tg-web: connecting to Telegram...');
    try {
      await withTimeout(client.connect(), 20_000, 'connect() to Telegram timed out after 20s — check TELEGRAM_API_ID/HASH and that this host can reach Telegram\'s servers (raw TCP, not just HTTPS)');
      this.log.info({ studioId }, 'tg-web: connected to Telegram MTProto layer');
    } catch (err) {
      this.log.error({ err: err.message, studioId }, 'tg-web: connect() failed');
      entry.status = 'error';
      entry.error = err.message;
      this.sessions.set(studioId, entry); // keep the entry so getStatus reports the error instead of 'none'
      throw err;
    }

    if (persistedSessionString) {
      // Resuming a previously-linked session — no QR needed if the session
      // is still valid. checkAuthorization confirms it hasn't been revoked
      // (e.g. the user logged out from a real Telegram client) before we
      // report 'connected'.
      const authorized = await client.checkAuthorization().catch(() => false);
      if (authorized) {
        await this._markConnected(studioId, entry);
        // A resumed session never re-runs history import — Telegram doesn't
        // push it again, and we already imported it once on first link.
        entry.historyState = 'inert';
        return;
      }
      this.log.warn({ studioId }, 'tg-web: persisted session no longer authorized, falling back to QR login');
      this.persisted.delete(studioId);
    }

    // client.signInUserWithQrCode(...) does NOT resolve just because a QR
    // was generated — it stays pending until the code is actually scanned
    // (or fails/cancelled), internally re-issuing a fresh token+callback
    // every ~30s as each one expires. Awaiting it inline here would block
    // _startSession — and therefore every caller of _ensureSession,
    // including the first getOrStartSession request — for as long as it
    // takes a human to pick up their phone and scan, which is not a
    // reasonable thing for a single HTTP request to sit on. Run it
    // detached instead: entry.qr/entry.status are mutated as side effects
    // from the qrCode/password callbacks below and picked up by polling
    // (getStatus / getOrStartSession's loop), exactly how wa-web's
    // Baileys equivalent already behaves (makeWASocket() returns
    // immediately; QR delivery is a separate event, not a blocking await).
    this._runQRLogin(studioId, client, entry).catch((err) => {
      this.log.error({ err: err.message, studioId }, 'tg-web: qr login failed (detached)');
    });
  }

  async _runQRLogin(studioId, client, entry) {
    this.log.info({ studioId }, 'tg-web: requesting qr login token');
    try {
      await client.signInUserWithQrCode(
        { apiId: this.apiId, apiHash: this.apiHash },
        {
          qrCode: async ({ token, expires }) => {
            const deepLink = 'tg://login?token=' + base64url(token);
            entry.qr = await QRCode.toDataURL(deepLink, { width: 300, margin: 2 });
            entry.status = 'qr';
            this.log.info({ studioId, expires }, 'tg-web: qr generated');
          },
          password: async (hint) => {
            entry.status = 'password_required';
            entry.passwordHint = hint || null;
            this.log.info({ studioId }, 'tg-web: 2FA password required');
            return new Promise((resolve) => {
              entry.resolvePassword = resolve;
            });
          },
          onError: async (err) => {
            this.log.error({ err: err.message, studioId }, 'tg-web: qr login error');
            return true; // stop the auth operation
          },
        },
      );
    } catch (err) {
      // Surfaced via getStatus as {status:'error', error} rather than
      // deleting the session outright — a poll landing right after this
      // would otherwise see 'none' (no session at all) and the frontend
      // would show a generic "service unavailable" instead of the real
      // reason (bad API ID/hash, network unreachable, user cancelled, etc).
      this.log.error({ err: err.message, studioId }, 'tg-web: qr login failed');
      entry.status = 'error';
      entry.error = err.message;
      await client.destroy().catch(() => {});
      throw err;
    }

    await this._markConnected(studioId, entry);
    this._runHistoryImport(studioId).catch((err) =>
      this.log.error({ err: err.message, studioId }, 'tg-web: initial history import failed'),
    );
  }

  async _markConnected(studioId, entry) {
    const me = await entry.client.getMe();
    entry.status = 'connected';
    entry.qr = null;
    entry.phone = me.phone ? `+${me.phone}` : null;
    entry.username = me.username || null;
    entry.sessionString = entry.client.session.save();
    this.persisted.set(studioId, entry.sessionString);
    this.log.info({ studioId, phone: entry.phone, username: entry.username }, 'tg-web: connected');

    entry.client.addEventHandler(async (event) => {
      const msg = event.message;
      if (!msg || msg.isGroup || msg.isChannel) return; // only direct chats, matches wa-web's group filtering
      if (!msg.text && !msg.media) return; // e.g. a service message (pin, join) — nothing to record

      let attachment = null;
      if (msg.media) {
        try {
          attachment = await this._downloadAndUploadMedia(entry.client, msg);
        } catch (err) {
          this.log.error({ err: err.message, studioId, messageId: msg.id }, 'tg-web: media download/upload failed — recording text only');
        }
      }

      await this._forwardInbound(studioId, {
        chatId: msg.chatId?.toString(),
        text: msg.text || '',
        messageId: String(msg.id),
        timestamp: msg.date,
        fromMe: !!msg.out,
        displayName: await senderDisplayName(msg),
        attachment,
      });
    }, new NewMessage({}));

    await this._notifyConnected(studioId, entry.phone, entry.username, entry.sessionString);
  }

  // Downloads an inbound attachment via GramJS (the only thing that can
  // reach MTProto media — see the comment on the Go side's tgWebMedia
  // handler) and pushes the bytes there to be saved under ./uploads,
  // getting back the URL to reference from the message row.
  async _downloadAndUploadMedia(client, msg) {
    const buffer = await client.downloadMedia(msg);
    if (!buffer || buffer.length === 0) return null;

    const { type, mimeType, fileName } = classifyMedia(msg);
    const form = new FormData();
    form.append('mimeType', mimeType || '');
    form.append('file', new Blob([buffer]), fileName || 'file');

    const res = await fetch(`${this.projectxApiUrl}/internal/tg-web/media`, { method: 'POST', body: form });
    if (!res.ok) throw new Error(`media upload failed: HTTP ${res.status}`);
    const body = await res.json();
    return { url: body.url, type, mime: mimeType, name: body.name };
  }

  async _forwardInbound(studioId, { chatId, text, messageId, timestamp, fromMe, displayName, attachment }) {
    try {
      const res = await fetch(`${this.projectxApiUrl}/internal/tg-web/inbound`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          studioId, chatId, text, messageId, timestamp, fromMe, displayName,
          attachmentUrl: attachment?.url, attachmentType: attachment?.type,
          attachmentMime: attachment?.mime, attachmentName: attachment?.name,
        }),
      });
      this.log.info({ studioId, chatId, status: res.status }, 'tg-web: forwarded inbound');
    } catch (err) {
      this.log.error({ err: err.message, studioId }, 'tg-web: forward inbound failed');
    }
  }

  // POSTs with a few retries — mirrors wa-web's _notifyWithRetry so a
  // transient Go-API restart at exactly the wrong moment doesn't leave
  // channel_accounts permanently out of sync.
  async _notifyWithRetry(path, body, { attempts = 4 } = {}) {
    for (let i = 1; i <= attempts; i++) {
      try {
        const res = await fetch(`${this.projectxApiUrl}${path}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        if (res.ok) return true;
        this.log.warn({ path, status: res.status, attempt: i }, 'tg-web: notify non-ok');
      } catch (err) {
        this.log.warn({ err: err.message, path, attempt: i }, 'tg-web: notify failed, retrying');
      }
      if (i < attempts) await sleep(i * 1000);
    }
    this.log.error({ path }, 'tg-web: notify failed after all retries');
    return false;
  }

  async _notifyConnected(studioId, phone, username, sessionString) {
    await this._notifyWithRetry('/internal/tg-web/connected', { studioId, phone, username, sessionString });
  }
}

async function resolveEntity(client, to) {
  // `to` is either a numeric chat/user id (string) as stored in
  // contact_identities, or (rarely, for a fresh outbound from the UI) a
  // @username.
  if (/^-?\d+$/.test(to)) return client.getEntity(BigInt(to));
  return client.getEntity(to.startsWith('@') ? to : `@${to}`);
}

// Resolves the human-readable name of whoever sent an inbound message.
// msg.sender is usually already populated from the entity cache that comes
// with the update; getSender() is the fallback for the rare case it isn't.
// Falls back through first+last name -> username -> undefined (letting the
// Go side's identity display_name stay empty rather than writing a
// meaningless placeholder).
async function senderDisplayName(msg) {
  const sender = msg.sender || (await msg.getSender().catch(() => undefined));
  if (!sender) return undefined;
  const fullName = [sender.firstName, sender.lastName].filter(Boolean).join(' ').trim();
  return fullName || sender.username || undefined;
}

// Classifies an inbound message's media into our generic Attachment.Type
// convention ("image"/"video"/"audio"/"document" — same vocabulary every
// other channel adapter uses) plus mime type and filename where Telegram
// exposes them. Message getters (msg.photo, msg.document, etc.) return
// undefined when that particular type isn't what this message carries —
// checked in specificity order since e.g. voice notes and audio files are
// both technically Api.Document under the hood, distinguished only by
// which named getter matches.
function classifyMedia(msg) {
  if (msg.photo) return { type: 'image', mimeType: 'image/jpeg', fileName: 'photo.jpg' };
  if (msg.videoNote) return { type: 'video', mimeType: msg.videoNote.mimeType, fileName: docFileName(msg.videoNote) };
  if (msg.video) return { type: 'video', mimeType: msg.video.mimeType, fileName: docFileName(msg.video) };
  if (msg.gif) return { type: 'video', mimeType: msg.gif.mimeType, fileName: docFileName(msg.gif) };
  if (msg.voice) return { type: 'audio', mimeType: msg.voice.mimeType || 'audio/ogg', fileName: docFileName(msg.voice) || 'voice.ogg' };
  if (msg.audio) return { type: 'audio', mimeType: msg.audio.mimeType, fileName: docFileName(msg.audio) };
  if (msg.sticker) return { type: 'image', mimeType: msg.sticker.mimeType || 'image/webp', fileName: docFileName(msg.sticker) || 'sticker.webp' };
  if (msg.document) return { type: 'document', mimeType: msg.document.mimeType, fileName: docFileName(msg.document) };
  return { type: 'document', mimeType: undefined, fileName: 'file' };
}

// Pulls the filename off an Api.Document's attributes array (the
// DocumentAttributeFilename entry, when present — voice notes and photos
// typically don't carry one).
function docFileName(doc) {
  const attr = doc?.attributes?.find((a) => typeof a?.fileName === 'string');
  return attr?.fileName;
}

function base64url(buf) {
  return Buffer.from(buf).toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

function withTimeout(promise, ms, message) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(message)), ms);
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}
