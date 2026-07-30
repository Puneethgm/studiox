import 'dotenv/config';
import express from 'express';
import pino from 'pino';
import { SessionManager } from './sessions.js';

const log = pino({ level: process.env.LOG_LEVEL || 'info' });
const app = express();
app.use(express.json());

const PORT = process.env.PORT || 3101;
const INTERNAL_API_KEY = process.env.INTERNAL_API_KEY;
if (!INTERNAL_API_KEY) { console.error('INTERNAL_API_KEY is not set'); process.exit(1); }
const PROJECTX_API_URL = process.env.PROJECTX_API_URL || 'http://api:8080';

const apiId = Number(process.env.TELEGRAM_API_ID);
const apiHash = process.env.TELEGRAM_API_HASH;
// Telegram QR-login is opt-in per deploy (unlike wa-web, which has no
// comparable platform-wide credential) — most contributors running `make
// dev` won't have these set, and this service is bundled into that default
// run. Degrade to "not configured" rather than exiting, so it doesn't
// crash-loop and spam everyone else's console; the bot-token Telegram
// channel doesn't depend on this service at all.
const configured = Boolean(apiId && apiHash);
if (!configured) {
  log.warn('TELEGRAM_API_ID / TELEGRAM_API_HASH not set — Telegram QR-login disabled. Set them in .env (see docs/SETUP_TELEGRAM_QR.md) to enable.');
}

const sessions = configured ? new SessionManager({ log, projectxApiUrl: PROJECTX_API_URL, apiId, apiHash }) : null;

// Auth middleware — all routes require the internal API key
app.use((req, res, next) => {
  if (req.headers['x-internal-key'] !== INTERNAL_API_KEY) {
    return res.status(401).json({ error: 'unauthorized' });
  }
  next();
});

// Short-circuits every /sessions/* route with a clear, consistent error
// when this deploy hasn't set up Telegram API credentials, instead of each
// handler individually null-checking `sessions`.
app.use('/sessions', (req, res, next) => {
  if (!configured) {
    return res.status(503).json({ status: 'not_configured', error: 'TELEGRAM_API_ID/TELEGRAM_API_HASH not set on this deploy' });
  }
  next();
});

// GET /sessions/:studioId/qr
// Returns the current QR code instantly if pre-warmed, or starts and waits.
app.get('/sessions/:studioId/qr', async (req, res) => {
  const { studioId } = req.params;
  try {
    const result = await sessions.getOrStartSession(studioId);
    res.json(result);
  } catch (err) {
    log.error({ err: err.message, studioId }, 'get qr failed');
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/password — submits a 2FA password when
// getOrStartSession/status reports status:'password_required'.
app.post('/sessions/:studioId/password', async (req, res) => {
  const { studioId } = req.params;
  const { password } = req.body;
  if (!password) return res.status(400).json({ error: 'password required' });
  try {
    const result = await sessions.submitPassword(studioId, password);
    res.json(result);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/disconnect
app.post('/sessions/:studioId/disconnect', async (req, res) => {
  const { studioId } = req.params;
  try {
    await sessions.disconnect(studioId);
    res.json({ ok: true });
  } catch (err) {
    log.error({ err: err.message, studioId }, 'disconnect failed');
    res.status(500).json({ error: err.message });
  }
});

// GET /sessions/:studioId/status
app.get('/sessions/:studioId/status', (req, res) => {
  const { studioId } = req.params;
  res.json(sessions.getStatus(studioId));
});

// POST /sessions/:studioId/send
app.post('/sessions/:studioId/send', async (req, res) => {
  const { studioId } = req.params;
  const { to, text, mediaUrl, mediaType, caption } = req.body;
  if (!to) return res.status(400).json({ error: 'to required' });
  try {
    let result;
    if (mediaUrl) {
      result = await sessions.sendMedia(studioId, to, mediaUrl, mediaType, caption || '');
    } else {
      if (!text) return res.status(400).json({ error: 'text required when no media' });
      result = await sessions.sendMessage(studioId, to, text);
    }
    res.json({ ok: true, messageId: String(result?.id ?? '') });
  } catch (err) {
    log.error({ err: err.message, studioId, to }, 'tg-web: send message failed');
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/prewarm — called by Go API when a studio connects,
// resumes a persisted session (does NOT start a fresh QR login unprompted).
app.post('/sessions/:studioId/prewarm', async (req, res) => {
  const { studioId } = req.params;
  sessions.prewarm(studioId).catch((err) => log.error({ err: err.message, studioId }, 'prewarm failed'));
  res.json({ ok: true });
});

// POST /sessions/:studioId/backfill — manual re-sync nudge from the admin UI.
app.post('/sessions/:studioId/backfill', async (req, res) => {
  const { studioId } = req.params;
  sessions.backfillHistory(studioId).catch((err) => log.error({ err: err.message, studioId }, 'backfill failed'));
  res.json({ ok: true, status: 'running' });
});

app.listen(PORT, () => {
  log.info({ port: PORT }, 'tg-web service started');
  if (configured) prewarmAll();
});

// Rehydrates persisted sessions from the Go API (each studio's encrypted
// session string, decrypted server-side before it reaches us — see
// GET /internal/tg-web/sessions in webhook_tg_web.go) so a tg-web restart
// resumes without every studio needing to re-scan a QR code. Unlike
// wa-web's Baileys auth (multi-file state on a Docker volume), a GramJS
// session is a single string, so the DB is the source of truth instead of
// a volume — one less stateful container to manage.
async function prewarmAll() {
  for (let attempt = 1; attempt <= 20; attempt++) {
    try {
      const res = await fetch(`${PROJECTX_API_URL}/internal/tg-web/sessions`, {
        headers: { 'x-internal-key': INTERNAL_API_KEY },
      });
      if (!res.ok) {
        log.warn({ status: res.status, attempt }, 'tg-web: pre-warm API not ready, retrying...');
        await sleep(2000);
        continue;
      }
      const { sessions: persisted } = await res.json();
      log.info({ count: persisted.length }, 'tg-web: pre-warming studios');
      for (const { studioId, sessionString } of persisted) {
        sessions.registerPersistedSession(studioId, sessionString);
        sessions.prewarm(studioId).catch((err) => log.error({ err: err.message, studioId }, 'prewarm failed'));
        await sleep(500);
      }
      return;
    } catch (err) {
      log.warn({ attempt }, 'tg-web: API not ready yet, retrying in 2s...');
      await sleep(2000);
    }
  }
  log.error('tg-web: pre-warm failed after all retries');
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}
