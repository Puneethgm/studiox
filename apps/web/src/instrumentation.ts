// Next.js's own instrumentation hook (https://nextjs.org/docs/app/building-your-application/optimizing/open-telemetry)
// — register() runs once, before anything else, on server start. Mirrors
// apps/api/internal/platform/tracing and apps/wa-web|tg-web/src/tracing.js:
// same OTel Collector -> Tempo -> Grafana pipeline, same no-op-when-unset
// contract. @vercel/otel handles Next.js's own instrumentation (fetch,
// Server Components/Actions) instead of a hand-rolled middleware, since
// there's no single Express-style middleware chain here to hook into.
//
// Previously: no structured logger at all (console.log/console.error
// only) — confirmed live this meant the "Up" dashboard tile could never
// go green for this service, since production `next start` prints
// nothing to stdout for a normal request, and there was nothing else to
// count. patchHttpAccessLog() below fixes that.
import { registerOTel, OTLPHttpJsonTraceExporter } from '@vercel/otel';

// deploy/web.Dockerfile runs Next's own generated `server.js` (standalone
// output) — not a file this repo writes, so there's no middleware chain
// of ours to hook into the way apps/api/wa-web/tg-web/embeddings do.
// Instead, monkey-patch Node's http.Server at the event-emitter level —
// the same technique OTel's own http auto-instrumentation uses — so
// every request through whatever server Next.js creates internally gets
// one structured access-log line, matching the other 4 services' field
// names (method/path/status/duration_ms/trace_id/span_id) exactly, for
// the "Recent requests" Loki table panel.
//
// `node:http` is dynamically imported, not a top-level one — Next.js
// bundles instrumentation.ts for BOTH the edge and nodejs runtimes, and
// the edge/webpack bundler can't resolve "node:http" at all. A plain
// `await import('node:http')` wasn't enough on its own either — webpack
// still eagerly resolves dynamic imports for its module graph at build
// time regardless of the runtime branch they're in (confirmed live: both
// attempts broke the production build with the same
// "UnhandledSchemeError: Reading from node:http is not handled by
// plugins"). The `webpackIgnore` magic comment is what actually works —
// it tells webpack to leave this import alone entirely and let Node
// resolve it at runtime, which is exactly what's needed since this
// branch only ever executes in the real nodejs runtime anyway.
async function patchHttpAccessLog() {
  const http = await import(/* webpackIgnore: true */ 'node:http');
  // Not webpackIgnore'd — this is a normal npm package, which webpack
  // resolves and bundles fine; only the "node:" scheme import above needs
  // the special-case.
  const { trace } = await import('@opentelemetry/api');

  const originalEmit = http.Server.prototype.emit;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (http.Server.prototype as any).emit = function (event: string, ...args: any[]) {
    if (event === 'request') {
      const req = args[0] as InstanceType<typeof http.IncomingMessage>;
      const res = args[1] as InstanceType<typeof http.ServerResponse>;
      const start = Date.now();
      res.on('finish', () => {
        const line: Record<string, unknown> = {
          time: new Date().toISOString(),
          level: 'INFO',
          msg: 'http_request',
          method: req.method,
          path: req.url?.split('?')[0],
          status: res.statusCode,
          duration_ms: Date.now() - start,
        };
        const span = trace.getActiveSpan();
        const sc = span?.spanContext();
        if (sc && trace.isSpanContextValid(sc)) {
          line.trace_id = sc.traceId;
          line.span_id = sc.spanId;
        }
        console.log(JSON.stringify(line));
      });
    }
    return originalEmit.apply(this, [event, ...args] as Parameters<typeof originalEmit>);
  };
}

export async function register() {
  // instrumentation.ts's register() runs once per runtime Next.js starts
  // (edge and nodejs separately) — the http module patch only makes
  // sense in the real Node process actually serving requests.
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    await patchHttpAccessLog();
  }

  const endpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
  if (!endpoint) return;

  registerOTel({
    serviceName: 'web',
    traceExporter: new OTLPHttpJsonTraceExporter({ url: endpoint }),
  });
}
