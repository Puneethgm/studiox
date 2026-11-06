import 'dotenv/config';
import express from 'express';
import pino from 'pino';
import path from 'path';
import { fileURLToPath } from 'url';
import { SessionManager } from './sessions.js';
import { initTracing, tracingMiddleware, traceLogMixin, accessLogMiddleware } from './tracing.js';
import { keyFromBase64 } from './encryptedAuthState.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Empty OTEL_EXPORTER_OTLP_ENDPOINT is a deliberate no-op (see tracing.js) —
// local dev without the observability stack running is unaffected.
const { tracer, shutdown: shutdownTracing } = initTracing('wa-web', process.env.OTEL_EXPORTER_OTLP_ENDPOINT || '');

// Also write to a plain file, not just stdout — stdout here is piped through
// `make dev`/concurrently into whatever terminal launched it, which isn't
// reliably readable after the fact (no scrollback access, no grep). The file
// gives a durable place to actually inspect Baileys' history-sync diagnostics
// (see sessions.js) without depending on someone copy-pasting terminal output.
const log = pino(
  {
    level: process.env.LOG_LEVEL || 'info',
    // Stamps trace_id/span_id (from tracingMiddleware's active span, see
    // below) onto every log line, the same way apps/api's logger.FromCtx
    // does for the Go side — lets a log line in Grafana link straight to
    // its Tempo trace.
    mixin: traceLogMixin,
  },
  pino.multistream([
    { stream: process.stdout },
    { stream: pino.destination({ dest: path.join(__dirname, '..', 'wa-web.log'), sync: false }) },
  ]),
);
const app = express();
// First in the chain so every downstream middleware/handler's logging runs
// inside the span and picks up its trace_id via the pino mixin above.
app.use(tracingMiddleware(tracer));
app.use(accessLogMiddleware(log));
app.use(express.json());

const PORT = process.env.PORT || 3100;
// Own secret, not the old shared INTERNAL_API_KEY — see
// platform-security-hardening task 0.4. Must match the Go API's
// WA_WEB_INTERNAL_KEY (apps/api/internal/messaging/webhook_wa_web.go).
const INTERNAL_API_KEY = process.env.WA_WEB_INTERNAL_KEY;
if (!INTERNAL_API_KEY) { console.error('WA_WEB_INTERNAL_KEY is not set'); process.exit(1); }
const PROJECTX_API_URL = process.env.PROJECTX_API_URL || 'http://api:8080';

// Encrypts Baileys' auth-state files at rest (creds, pre-keys, identity
// keys, session keys) — previously written as plaintext JSON. Required;
// no fallback (platform-security-hardening task 0.5). 32 bytes,
// base64-encoded — generate with: openssl rand -base64 32
if (!process.env.WA_WEB_SESSION_ENCRYPTION_KEY) {
  console.error('WA_WEB_SESSION_ENCRYPTION_KEY is not set');
  process.exit(1);
}
const SESSION_ENCRYPTION_KEY = keyFromBase64(process.env.WA_WEB_SESSION_ENCRYPTION_KEY);

const sessions = new SessionManager({ log, projectxApiUrl: PROJECTX_API_URL, sessionEncryptionKey: SESSION_ENCRYPTION_KEY });

// Idempotency cache for /send — the Go API retries a send job whenever *its*
// HTTP call to us times out or drops, which can happen after we've already
// dispatched the message to WhatsApp (sending here can take a while; the
// caller's wait doesn't). Without this, a retry would re-send the same
// message to the customer a second time. Keyed by the caller-supplied
// idempotencyKey (one per outbound job + attachment index — see
// waWebSender.SendText on the Go side), value is either a Promise (send in
// flight — a genuinely concurrent retry awaits the same send instead of
// starting a second one) or a {messageId, expiresAt} record once it
// resolves. A failed send is removed, not cached, so it can be retried for
// real. The normal job-level backoff in worker.go only spends ~1.5h total
// retrying a single job — but a 2026-10 bug on the Go side (a foreign-key
// violation on the "already sent, deduped" path — see MarkOutboundSent's
// doc comment) kept jobs stuck in 'pending' and retrying indefinitely,
// well past that budget, which once silently exceeded this cache's old 2h
// TTL and let a real duplicate through. That Go-side bug is now fixed, but
// 24h is kept as a deliberately generous safety margin regardless, since a
// cache miss here has real customer-facing consequences.
const IDEMPOTENCY_TTL_MS = 24 * 60 * 60 * 1000;
const idempotencyCache = new Map();
setInterval(() => {
  const now = Date.now();
  for (const [key, entry] of idempotencyCache) {
    if (entry.expiresAt && entry.expiresAt < now) idempotencyCache.delete(key);
  }
}, 10 * 60 * 1000).unref();

// Runs `send()` for this idempotencyKey at most once; a concurrent or later
// call with the same key gets the same result instead of triggering another
// real WhatsApp send. Falls back to always sending when no key is given.
async function sendIdempotent(idempotencyKey, send) {
  if (!idempotencyKey) return send();

  const existing = idempotencyCache.get(idempotencyKey);
  if (existing) {
    if (existing.messageId !== undefined) return { key: { id: existing.messageId } };
    return existing.promise; // already in flight — await the same send
  }

  const promise = send();
  idempotencyCache.set(idempotencyKey, { promise });
  try {
    const result = await promise;
    idempotencyCache.set(idempotencyKey, {
      messageId: result?.key?.id ?? null,
      expiresAt: Date.now() + IDEMPOTENCY_TTL_MS,
    });
    return result;
  } catch (err) {
    idempotencyCache.delete(idempotencyKey); // let a real failure be retried
    throw err;
  }
}

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

// GET /sessions/:studioId/check/:number — diagnostic: is this number
// actually registered on WhatsApp? (see SessionManager.checkOnWhatsApp)
app.get('/sessions/:studioId/check/:number', async (req, res) => {
  const { studioId, number } = req.params;
  try {
    const results = await sessions.checkOnWhatsApp(studioId, number);
    res.json({ results });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/send
app.post('/sessions/:studioId/send', async (req, res) => {
  const { studioId } = req.params;
  const { to, text, mediaUrl, mediaType, caption, idempotencyKey } = req.body;
  if (!to) return res.status(400).json({ error: 'to required' });
  if (!mediaUrl && !text) return res.status(400).json({ error: 'text required when no media' });
  if (idempotencyKey && idempotencyCache.has(idempotencyKey) && idempotencyCache.get(idempotencyKey).messageId !== undefined) {
    log.info({ studioId, to, idempotencyKey }, 'wa-web: send already completed for this job — returning cached result, not re-sending');
  } else {
    log.info({ studioId, to, hasMedia: !!mediaUrl, textLen: text?.length, idempotencyKey }, 'wa-web: sending message');
  }
  try {
    const result = await sendIdempotent(idempotencyKey, () =>
      mediaUrl
        ? sessions.sendMedia(studioId, to, mediaUrl, mediaType, caption || '')
        : sessions.sendMessage(studioId, to, text)
    );
    log.info({ studioId, to }, 'wa-web: message sent ok');
    res.json({ ok: true, messageId: result?.key?.id });
  } catch (err) {
    log.error({ err: err.message, studioId, to }, 'wa-web: send message failed');
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/delete-message — real WhatsApp "delete for
// everyone", not a local hide. See SessionManager.deleteMessage's own doc
// comment for the real constraints (own messages only, WhatsApp's own time
// window, not something this code enforces or can override).
app.post('/sessions/:studioId/delete-message', async (req, res) => {
  const { studioId } = req.params;
  const { to, messageId } = req.body;
  if (!to) return res.status(400).json({ error: 'to required' });
  if (!messageId) return res.status(400).json({ error: 'messageId required' });
  log.info({ studioId, to, messageId }, 'wa-web: deleting message');
  try {
    await sessions.deleteMessage(studioId, to, messageId);
    log.info({ studioId, to, messageId }, 'wa-web: message deleted ok');
    res.json({ ok: true });
  } catch (err) {
    log.error({ err: err.message, studioId, to, messageId }, 'wa-web: delete message failed');
    res.status(500).json({ error: err.message });
  }
});

// POST /sessions/:studioId/prewarm — called by Go API when a studio connects
app.post('/sessions/:studioId/prewarm', async (req, res) => {
  const { studioId } = req.params;
  sessions.prewarm(studioId).catch(err => log.error({ err, studioId }, 'prewarm failed'));
  res.json({ ok: true });
});

// POST /sessions/:studioId/backfill — manual nudge from the admin UI. The
// actual import runs automatically in the background as soon as a session
// connects (see SessionManager._startSession / _concludeHistoryImport); this
// just re-syncs the Go API's view of the current state (see
// SessionManager.backfillHistory for the exact cases it covers).
app.post('/sessions/:studioId/backfill', async (req, res) => {
  const { studioId } = req.params;
  sessions.backfillHistory(studioId).catch(err => log.error({ err, studioId }, 'backfill failed'));
  res.json({ ok: true, status: 'running' });
});

app.listen(PORT, () => {
  log.info({ port: PORT }, 'wa-web service started');
  // Pre-warm Chrome for all studios that have a whatsapp_web channel.
  // This runs in background — service is immediately ready to handle requests.
  prewarmAll();
  // Backstop so channel_accounts.status can't permanently drift out of sync
  // with the real session state if a connect/disconnect notification was
  // ever missed (e.g. the Go API was briefly down).
  sessions.startStatusReconciliation();
});

// Flushes any trace spans still buffered in the BatchSpanProcessor —
// without this, a container stop (SIGTERM, the normal `docker compose
// down`/restart path) can drop the last batch instead of exporting it.
for (const sig of ['SIGTERM', 'SIGINT']) {
  process.on(sig, async () => {
    await shutdownTracing().catch((err) => log.error({ err }, 'tracing shutdown failed'));
    process.exit(0);
  });
}

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
