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

  async sendMessage(studioId, to, text) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    // If already a full WA chat ID (has @), try direct send first
    if (to.includes('@')) {
      try {
        return await s.client.sendMessage(to, text);
      } catch (err) {
        if (!err.message?.includes('LID')) throw err;
        // LID error on @c.us — this contact requires @lid routing.
        // Try the same numeric part with @lid suffix.
        if (to.endsWith('@c.us')) {
          const number = to.replace('@c.us', '');
          try {
            return await s.client.sendMessage(number + '@lid', text);
          } catch (_) {}
        }
        // Fall through to getNumberId resolution
      }
    }
    // Resolve via getNumberId (works for regular numbers)
    const number = to.replace(/[^\d]/g, '');
    const numberId = await s.client.getNumberId(number);
    if (numberId) {
      return await s.client.sendMessage(numberId._serialized, text);
    }
    // Last resort: try @lid directly
    return await s.client.sendMessage(number + '@lid', text);
  }

  async sendMedia(studioId, to, mediaUrl, mediaType, caption) {
    const s = this.sessions.get(studioId);
    if (!s || s.status !== 'connected') throw new Error(`session not connected for studio ${studioId}`);
    const absoluteUrl = mediaUrl.startsWith('http') ? mediaUrl : `${this.projectxApiUrl}${mediaUrl}`;
    const media = await MessageMedia.fromUrl(absoluteUrl, { unsafeMime: true });

    // Try sending — with the same @lid fallback as sendMessage
    if (to.includes('@')) {
      try {
        return await s.client.sendMessage(to, media, { caption });
      } catch (err) {
        if (!err.message?.includes('LID')) throw err;
        if (to.endsWith('@c.us')) {
          const number = to.replace('@c.us', '');
          try {
            return await s.client.sendMessage(number + '@lid', media, { caption });
          } catch (_) {}
        }
      }
    }
    const number = to.replace(/[^\d]/g, '');
    const numberId = await s.client.getNumberId(number);
    if (numberId) return await s.client.sendMessage(numberId._serialized, media, { caption });
    return await s.client.sendMessage(number + '@lid', media, { caption });
  }

  // Pre-warm Chrome for a studio so the QR is ready before the user clicks.
  async prewarm(studioId) {
    if (this.sessions.has(studioId)) return;
    this.log.info({ studioId }, 'wa-web: pre-warming session');
    await this._startSession(studioId);
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
      // Resolve phone number from LID if needed
      if (msg.from.endsWith('@lid')) {
        try {
          const contact = await msg.getContact();
          if (contact?.number) {
            msg._resolvedPhone = contact.number;
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

  async _notifyConnected(studioId, phone) {
    try {
      const res = await fetch(`${this.projectxApiUrl}/internal/wa-web/connected`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId, phone }),
      });
      if (!res.ok) this.log.warn({ studioId, status: res.status }, 'wa-web: notify connected non-ok');
    } catch (err) {
      this.log.error({ err, studioId }, 'wa-web: notify connected failed');
    }
  }

  async _notifyDisconnected(studioId) {
    try {
      await fetch(`${this.projectxApiUrl}/internal/wa-web/disconnected`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ studioId }),
      });
    } catch (err) {
      this.log.error({ err, studioId }, 'wa-web: notify disconnected failed');
    }
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
