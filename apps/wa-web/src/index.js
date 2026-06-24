import 'dotenv/config';
import express from 'express';
import pino from 'pino';
import { SessionManager } from './sessions.js';

const log = pino({ level: process.env.LOG_LEVEL || 'info' });
const app = express();
app.use(express.json());

const PORT = process.env.PORT || 3100;
const INTERNAL_API_KEY = process.env.INTERNAL_API_KEY || 'changeme';
const PROJECTX_API_URL = process.env.PROJECTX_API_URL || 'http://api:8080';

const sessions = new SessionManager({ log, projectxApiUrl: PROJECTX_API_URL });

// Auth middleware — all routes require the internal API key
app.use((req, res, next) => {
  if (req.headers['x-internal-key'] !== INTERNAL_API_KEY) {
    return res.status(401).json({ error: 'unauthorized' });
  }
  next();
});

// GET /sessions/:studioId/qr
// Returns the current QR code instantly if pre-warmed, or starts and waits.
app.get('/sessions/:studioId/qr', async (req, res) => {
  const { studioId } = req.params;
  try {
    const result = await sessions.getOrStartSession(studioId);
    if (result.status === 'connected') {
      return res.json({ status: 'connected', phone: result.phone });
    }
    if (result.status === 'qr' && result.qr) {
      return res.json({ status: 'qr', qr: result.qr });
    }
    return res.json({ status: 'pending' });
  } catch (err) {
    log.error({ err, studioId }, 'get qr failed');
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
    log.error({ err, studioId }, 'disconnect failed');
    res.status(500).json({ error: err.message });
  }
});

// GET /sessions/:studioId/status
app.get('/sessions/:studioId/status', (req, res) => {
  const { studioId } = req.params;
  const status = sessions.getStatus(studioId);
  res.json(status);
});

// POST /sessions/:studioId/send
app.post('/sessions/:studioId/send', async (req, res) => {
  const { studioId } = req.params;
  const { to, text, mediaUrl, mediaType, caption } = req.body;
  if (!to) return res.status(400).json({ error: 'to required' });
  log.info({ studioId, to, hasMedia: !!mediaUrl, textLen: text?.length }, 'wa-web: sending message');
  try {
    let result;
    if (mediaUrl) {
      result = await sessions.sendMedia(studioId, to, mediaUrl, mediaType, caption || '');
    } else {
      if (!text) return res.status(400).json({ error: 'text required when no media' });
      result = await sessions.sendMessage(studioId, to, text);
    }
    log.info({ studioId, to }, 'wa-web: message sent ok');
    res.json({ ok: true, messageId: result?.key?.id });
  } catch (err) {
    log.error({ err: err.message, studioId, to }, 'wa-web: send message failed');
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/prewarm — called by Go API when a studio connects
app.post('/sessions/:studioId/prewarm', async (req, res) => {
  const { studioId } = req.params;
  sessions.prewarm(studioId).catch(err => log.error({ err, studioId }, 'prewarm failed'));
  res.json({ ok: true });
});

app.listen(PORT, () => {
  log.info({ port: PORT }, 'wa-web service started');
  // Pre-warm Chrome for all studios that have a whatsapp_web channel.
  // This runs in background — service is immediately ready to handle requests.
  prewarmAll();
});

async function prewarmAll() {
  // Retry until the Go API is ready (it starts concurrently and may not be up yet)
  for (let attempt = 1; attempt <= 20; attempt++) {
    try {
      const res = await fetch(`${PROJECTX_API_URL}/internal/wa-web/studios`, {
        headers: { 'x-internal-key': INTERNAL_API_KEY },
      });
      if (!res.ok) {
        log.warn({ status: res.status, attempt }, 'wa-web: pre-warm API not ready, retrying...');
        await sleep(2000);
        continue;
      }
      const { studioIds } = await res.json();
      log.info({ count: studioIds.length }, 'wa-web: pre-warming studios');
      for (const id of studioIds) {
        sessions.prewarm(id).catch(err => log.error({ err, studioId: id }, 'prewarm failed'));
        await sleep(500);
      }
      return;
    } catch (err) {
      log.warn({ attempt }, 'wa-web: API not ready yet, retrying in 2s...');
      await sleep(2000);
    }
  }
  log.error('wa-web: pre-warm failed after all retries');
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}
