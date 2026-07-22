import pkg from 'whatsapp-web.js';
const { Client, LocalAuth, MessageMedia } = pkg;
import QRCode from 'qrcode';
import { mkdirSync, rmSync } from 'fs';
import { execSync } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const AUTH_DIR = process.env.AUTH_DIR || path.join(__dirname, '..', 'auth');

export class SessionManager {
  constructor({ log, projectxApiUrl }) {
    this.log = log;
    this.projectxApiUrl = projectxApiUrl;
    // Map<studioId, { client, qr, status, phone }>
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
    // Wait up to 30s for Chrome to start and QR to appear
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
    if (s?.client) {
      await s.client.logout().catch(() => {});
      await s.client.destroy().catch(() => {});
    }
    this.sessions.delete(studioId);
  }

  _isDeadSession(err) {
    const msg = err?.message || '';
    return msg.includes('detached Frame') || msg.includes('Target closed') ||
      msg.includes('Session closed') || msg.includes('Protocol error');
  }

  async _recoverSession(studioId) {
    this.log.warn({ studioId }, 'wa-web: dead session detected, restarting...');
    const s = this.sessions.get(studioId);
    if (s?.client) await s.client.destroy().catch(() => {});
    this.sessions.delete(studioId);
    await this._notifyDisconnected(studioId);
    this._startSession(studioId).catch(err =>
      this.log.error({ err, studioId }, 'wa-web: session recovery failed'),
    );
  }

  async sendMessage(studioId, to, text) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    try {
      // If already a full WA chat ID (has @), try direct send first
      if (to.includes('@')) {
        try {
          return await s.client.sendMessage(to, text);
        } catch (err) {
          if (this._isDeadSession(err)) { await this._recoverSession(studioId); throw err; }
          if (!err.message?.includes('LID')) throw err;
          if (to.endsWith('@c.us')) {
            const number = to.replace('@c.us', '');
            try { return await s.client.sendMessage(number + '@lid', text); } catch (_) {}
          }
        }
      }
      const number = to.replace(/[^\d]/g, '');
      const numberId = await s.client.getNumberId(number);
      if (numberId) return await s.client.sendMessage(numberId._serialized, text);
      return await s.client.sendMessage(number + '@lid', text);
    } catch (err) {
      if (this._isDeadSession(err)) await this._recoverSession(studioId);
      throw err;
    }
  }

  async sendMedia(studioId, to, mediaUrl, mediaType, caption) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const absoluteUrl = mediaUrl.startsWith('http') ? mediaUrl : `${this.projectxApiUrl}${mediaUrl}`;
    const media = await MessageMedia.fromUrl(absoluteUrl, { unsafeMime: true });

    try {
      if (to.includes('@')) {
        try {
          return await s.client.sendMessage(to, media, { caption });
        } catch (err) {
          if (this._isDeadSession(err)) { await this._recoverSession(studioId); throw err; }
          if (!err.message?.includes('LID')) throw err;
          if (to.endsWith('@c.us')) {
            const number = to.replace('@c.us', '');
            try { return await s.client.sendMessage(number + '@lid', media, { caption }); } catch (_) {}
          }
        }
      }
      const number = to.replace(/[^\d]/g, '');
      const numberId = await s.client.getNumberId(number);
      if (numberId) return await s.client.sendMessage(numberId._serialized, media, { caption });
      return await s.client.sendMessage(number + '@lid', media, { caption });
    } catch (err) {
      if (this._isDeadSession(err)) await this._recoverSession(studioId);
      throw err;
    }
  }

  // Pre-warm Chrome for a studio so the QR is ready before the user clicks.
  async prewarm(studioId) {
    if (this.sessions.has(studioId)) return;
    this.log.info({ studioId }, 'wa-web: pre-warming session');
    await this._startSession(studioId);
  }

  // Imports existing chat history after a QR link, best-effort. WhatsApp Web
  // only syncs a limited recent window to linked (non-primary) devices by
  // default — syncHistory() asks the server to push more down for chats that
  // aren't fully synced yet, then fetchMessages() reads what's available.
  // Text-only for now (media forwarding isn't implemented on the inbound
  // side). Reports progress and completion back to the Go API as it goes,
  // one chat at a time, so a huge contact list can't block the shared
  // Puppeteer page indefinitely without any visibility.
  async backfillHistory(studioId, { perChatLimit = 200, maxChats = 50 } = {}) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    if (s.backfillRunning) {
      this.log.warn({ studioId }, 'wa-web: backfill already running, ignoring duplicate request');
      return;
    }
    s.backfillRunning = true;
    const { client } = s;

    let totalImported = 0;
    let failed = false;
    try {
      // WhatsApp's internal chat store isn't always hydrated the instant
      // 'ready' fires, especially on a freshly linked device — getChats()
      // can throw a minified internal error (e.g. err.message === 'r') if
      // called too early. Retry a few times with a short backoff before
      // giving up.
      let chats;
      let lastErr;
      for (let attempt = 1; attempt <= 5; attempt++) {
        try {
          chats = await client.getChats();
          lastErr = null;
          break;
        } catch (err) {
          lastErr = err;
          if (this._isDeadSession(err)) throw err;
          this.log.warn(
            { err: { message: err.message, name: err.name }, studioId, attempt },
            'wa-web: getChats failed, retrying',
          );
          await sleep(3000);
        }
      }
      if (lastErr) throw lastErr;

      const toProcess = chats.slice(0, maxChats);
      this.log.info({ studioId, chatCount: chats.length, processing: toProcess.length }, 'wa-web: backfill starting');

      for (const chat of toProcess) {
        try {
          if (chat.endOfHistoryTransferType === 0) {
            await chat.syncHistory().catch(() => {});
            await sleep(2000);
          }

          // Resolve @lid chat IDs to real phone numbers, same as the live
          // message path (_forwardInbound) — otherwise the inbox shows
          // WhatsApp's opaque Linked-ID instead of a phone number. Group
          // chats (@g.us) have no phone number and are left as-is.
          let chatFrom = chat.id._serialized;
          if (!chat.isGroup && chatFrom.endsWith('@lid')) {
            try {
              const contact = await chat.getContact();
              // contact.number mirrors the LID itself for @lid-only contacts —
              // the real phone number instead comes back as contact.id
              // (a @c.us wid) once WhatsApp resolves it.
              const resolvedId = contact?.id?._serialized;
              if (resolvedId && resolvedId.endsWith('@c.us')) {
                chatFrom = resolvedId;
              }
            } catch (_) {
              // couldn't resolve — fall back to the raw @lid id
            }
          }

          const history = await chat.fetchMessages({ limit: perChatLimit });
          const messages = history
            .filter(msg => !!msg.body)
            .map(msg => ({
              from: chatFrom,
              text: msg.body,
              messageId: msg.id._serialized,
              timestamp: msg.timestamp,
              fromMe: msg.fromMe,
            }));
          if (messages.length === 0) continue;

          const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/backfill`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ studioId, messages }),
          });
          if (res.ok) {
            const body = await res.json().catch(() => ({}));
            totalImported += body.imported || 0;
          } else {
            this.log.warn({ studioId, chatId: chat.id._serialized, status: res.status }, 'wa-web: backfill chat push non-ok');
          }
        } catch (err) {
          this.log.warn({ err: err.message, studioId, chatId: chat.id?._serialized }, 'wa-web: backfill chat failed, continuing');
        }
      }
    } catch (err) {
      this.log.error(
        { err: { message: err.message, name: err.name, stack: err.stack }, studioId },
        'wa-web: backfill failed',
      );
      failed = true;
      if (this._isDeadSession(err)) await this._recoverSession(studioId);
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

  async _startSession(studioId) {
    const dataPath = path.join(AUTH_DIR, studioId);
    mkdirSync(dataPath, { recursive: true });

    // Kill any orphaned Chrome process holding the session directory lock, then
    // remove the lock files so a fresh Chrome can always start cleanly.
    const sessionDir = path.join(AUTH_DIR, `session-${studioId}`);
    try {
      execSync(`pkill -f "${sessionDir}" 2>/dev/null || true`);
      await sleep(500);
    } catch (_) {}
    for (const lock of ['SingletonLock', 'SingletonSocket', 'SingletonCookie']) {
      rmSync(path.join(sessionDir, lock), { force: true });
    }

    const entry = { status: 'pending', qr: null, phone: null, client: null };
    this.sessions.set(studioId, entry);

    const client = new Client({
      authStrategy: new LocalAuth({
        clientId: studioId,
        dataPath: AUTH_DIR,
      }),
      puppeteer: {
        headless: true,
        executablePath: '/opt/google/chrome/chrome',
        args: [
          '--no-sandbox',
          '--disable-setuid-sandbox',
          '--disable-dev-shm-usage',
          '--disable-accelerated-2d-canvas',
          '--no-first-run',
          '--no-zygote',
          '--disable-gpu',
          '--window-size=1280,800',
        ],
      },
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36',
    });

    entry.client = client;

    client.on('qr', async (qr) => {
      try {
        const qrPng = await QRCode.toDataURL(qr, { width: 300, margin: 2 });
        entry.qr = qrPng;
        entry.status = 'qr';
        this.log.info({ studioId }, 'wa-web: qr generated');
      } catch (err) {
        this.log.error({ err }, 'wa-web: qrcode generation failed');
      }
    });

    client.on('ready', async () => {
      const info = client.info;
      const phone = info?.wid?.user || '';
      entry.status = 'connected';
      entry.qr = null;
      entry.phone = phone;
      this.log.info({ studioId, phone }, 'wa-web: connected');
      await this._notifyConnected(studioId, phone);
    });

    client.on('authenticated', () => {
      entry.status = 'pending';
      entry.qr = null;
      this.log.info({ studioId }, 'wa-web: authenticated, loading...');
    });

    client.on('auth_failure', (msg) => {
      this.log.error({ studioId, msg }, 'wa-web: auth failure');
      entry.status = 'pending';
      entry.qr = null;
    });

    client.on('disconnected', async (reason) => {
      this.log.warn({ studioId, reason }, 'wa-web: disconnected');
      entry.status = 'pending';
      entry.qr = null;
      this.sessions.delete(studioId);
      await client.destroy().catch(() => {});
      await this._notifyDisconnected(studioId);
      // Re-prewarm immediately so Chrome is ready for the next QR scan.
      this._startSession(studioId).catch(err =>
        this.log.error({ err, studioId }, 'wa-web: re-prewarm after disconnect failed'),
      );
    });

    client.on('message', async (msg) => {
      this.log.info({ studioId, from: msg.from, body: msg.body?.slice(0, 50) }, 'wa-web: message received');
      if (msg.fromMe) return;
      // Resolve phone number from LID if needed. contact.number mirrors the
      // LID itself for @lid-only contacts — the real phone number instead
      // comes back as contact.id (a @c.us wid) once WhatsApp resolves it.
      if (msg.from.endsWith('@lid')) {
        try {
          const contact = await msg.getContact();
          const resolvedId = contact?.id?._serialized;
          if (resolvedId && resolvedId.endsWith('@c.us')) {
            msg._resolvedPhone = contact.id.user;
          }
        } catch (e) {
          this.log.warn({ studioId, from: msg.from }, 'wa-web: could not resolve phone from LID');
        }
      }
      await this._forwardInbound(studioId, msg);
    });

    // Initialize (starts Puppeteer + Chromium), retry once then clear stuck session
    client.initialize().catch(async (err) => {
      this.log.error({ err, studioId }, 'wa-web: initialize failed, retrying in 5s');
      await sleep(5000);
      client.initialize().catch((err2) => {
        this.log.error({ err: err2, studioId }, 'wa-web: initialize retry failed — clearing stuck session');
        this.sessions.delete(studioId);
      });
    });
  }

  // POSTs with a few retries (backing off) so a transient Go-API restart
  // right when a session connects/disconnects doesn't leave channel_accounts
  // permanently out of sync with the real session state. Each attempt is
  // logged; a final failure is logged as an error but never throws — the
  // periodic reconciliation loop (see _reconcileStatuses) is the backstop
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

  async _forwardInbound(studioId, msg) {
    try {
      // Use resolved phone number if available, otherwise keep the original chat ID
      const from = msg._resolvedPhone
        ? msg._resolvedPhone + '@c.us'
        : msg.from;
      const text = msg.body || '';
      if (!text) return;
      const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/inbound`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          studioId,
          from,
          text,
          messageId: msg.id._serialized,
          timestamp: msg.timestamp,
        }),
      });
      const body = await res.text();
      this.log.info({ studioId, from, status: res.status, body }, 'wa-web: forwarded inbound');
    } catch (err) {
      this.log.error({ err, studioId }, 'wa-web: forward inbound failed');
    }
  }
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}
