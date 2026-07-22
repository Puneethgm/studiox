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
const BACKFILL_WAIT_MS = 10_000;

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

  // Baileys pushes chat history automatically via the 'messaging-history.set'
  // socket event shortly after a fresh pairing (see the listener registered
  // in _startSession, which buffers it into entry.historyBuffer). This just
  // replays whatever's already buffered — or waits briefly for it to arrive
  // if the request comes in right after connecting — instead of pulling on
  // demand the way the old whatsapp-web.js version did.
  async backfillHistory(studioId) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    if (s.backfillRunning) {
      this.log.warn({ studioId }, 'wa-web: backfill already running, ignoring duplicate request');
      return;
    }
    s.backfillRunning = true;

    let totalImported = 0;
    let failed = false;
    try {
      const waited = await this._waitForHistoryBuffer(studioId, BACKFILL_WAIT_MS);
      if (!waited) {
        this.log.warn({ studioId }, 'wa-web: backfill — no history-sync arrived in time');
        failed = true;
      } else {
        const current = this.sessions.get(studioId);
        const chatEntries = [...(current?.historyBuffer?.entries() ?? [])].slice(0, BACKFILL_MAX_CHATS);
        this.log.info(
          { studioId, chatCount: current?.historyBuffer?.size ?? 0, processing: chatEntries.length },
          'wa-web: backfill starting',
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
              this.log.warn({ studioId, status: res.status }, 'wa-web: backfill chat push non-ok');
            }
          } catch (err) {
            this.log.warn({ err: err.message, studioId }, 'wa-web: backfill chat failed, continuing');
          }
        }
      }
    } catch (err) {
      this.log.error({ err: { message: err.message, stack: err.stack }, studioId }, 'wa-web: backfill failed');
      failed = true;
    } finally {
      const running = this.sessions.get(studioId);
      if (running) running.backfillRunning = false;
    }

    try {
      await fetch(`${this.projectxApiUrl}/internal/wa-web/backfill-done`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId, messageCount: totalImported, failed }),
      });
    } catch (err) {
      this.log.error({ err, studioId }, 'wa-web: notify backfill done failed');
    }
    this.log.info({ studioId, totalImported, failed }, 'wa-web: backfill finished');
  }

  async _waitForHistoryBuffer(studioId, maxWaitMs) {
    const start = Date.now();
    while (Date.now() - start < maxWaitMs) {
      const s = this.sessions.get(studioId);
      if (s?.historyBuffer && s.historyBuffer.size > 0) return true;
      await sleep(500);
    }
    const s = this.sessions.get(studioId);
    return !!(s?.historyBuffer && s.historyBuffer.size > 0);
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

    const entry = { status: 'pending', qr: null, phone: null, sock: null, historyBuffer: new Map(), reconnectAttempts };
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
        return;
      }

      if (connection === 'close') {
        const statusCode = lastDisconnect?.error?.output?.statusCode;
        const loggedOut = statusCode === DisconnectReason.loggedOut;
        this.log.warn({ studioId, statusCode, loggedOut }, 'wa-web: disconnected');

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
        const text = msg.message?.conversation || msg.message?.extendedTextMessage?.text || '';
        this.log.info({ studioId, from: msg.key?.remoteJid, body: text.slice(0, 50) }, 'wa-web: message received');
        if (!text) continue;
        await this._forwardInbound(studioId, {
          from: normalizeJid(msg.key.remoteJid),
          text,
          messageId: msg.key.id,
          timestamp: Number(msg.messageTimestamp) || Math.floor(Date.now() / 1000),
        });
      }
    });

    // Buffer Baileys' automatic post-pairing history push. Fires once
    // (possibly in a few chunks, `syncType`-dependent) shortly after a fresh
    // QR link. See backfillHistory() for how this is replayed to the Go API.
    sock.ev.on('messaging-history.set', ({ messages }) => {
      if (!messages?.length) return;
      for (const msg of messages) {
        if (msg.key?.fromMe) continue;
        const text = msg.message?.conversation || msg.message?.extendedTextMessage?.text || '';
        if (!text) continue;
        const jid = normalizeJid(msg.key.remoteJid);
        if (!jid || jid.endsWith('@g.us')) continue; // skip group chats, matches old behavior
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

// Normalizes a Baileys JID to the @c.us / @lid convention the Go API's
// identity-stitching logic already expects (it previously only ever saw
// whatsapp-web.js's addressing scheme).
function normalizeJid(jid) {
  if (!jid) return jid;
  if (jid.endsWith('@s.whatsapp.net')) return jid.replace('@s.whatsapp.net', '@c.us');
  return jid; // @lid and @g.us pass through as-is
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
