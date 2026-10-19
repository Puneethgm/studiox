// Same OTel wiring as apps/wa-web/src/tracing.js — one span per HTTP
// request, exported to the same otel-collector -> Tempo -> Grafana
// pipeline used by the Go API. See that file's comment for why this is
// manual instrumentation rather than auto-instrumentation (ESM module
// patching needs a --import loader hook; a single startSpan()/context.with()
// middleware avoids that entirely).
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import { AsyncHooksContextManager } from '@opentelemetry/context-async-hooks';
import { trace, context, SpanStatusCode, SpanKind } from '@opentelemetry/api';

/**
 * @param {string} serviceName
 * @param {string} otlpEndpoint - e.g. "http://otel-collector:4318/v1/traces". Empty disables tracing (no-op).
 * @returns {{ tracer: import('@opentelemetry/api').Tracer, shutdown: () => Promise<void> }}
 */
export function initTracing(serviceName, otlpEndpoint) {
  if (!otlpEndpoint) {
    return { tracer: trace.getTracer(serviceName), shutdown: async () => {} };
  }

  const contextManager = new AsyncHooksContextManager();
  contextManager.enable();
  context.setGlobalContextManager(contextManager);

  const provider = new NodeTracerProvider({
    resource: resourceFromAttributes({ [ATTR_SERVICE_NAME]: serviceName }),
    spanProcessors: [new BatchSpanProcessor(new OTLPTraceExporter({ url: otlpEndpoint }))],
  });
  provider.register();

  return { tracer: trace.getTracer(serviceName), shutdown: () => provider.shutdown() };
}

/** Express middleware: starts one span per request, runs the rest of the chain inside it. */
export function tracingMiddleware(tracer) {
  return (req, res, next) => {
    // SpanKind.SERVER matters beyond labeling: Tempo's span-metrics/
    // service-graph processors classify edges by span kind — without this
    // every span defaults to INTERNAL and never contributes correctly to
    // the Service Graph, even once two services are actually propagating
    // trace context to each other (not yet true for this stack — see
    // deploy/README.md's Observability section).
    const span = tracer.startSpan(`${req.method} ${req.path}`, { kind: SpanKind.SERVER });
    span.setAttribute('http.method', req.method);
    span.setAttribute('http.target', req.originalUrl || req.path);
    const ctx = trace.setSpan(context.active(), span);
    res.on('finish', () => {
      span.setAttribute('http.status_code', res.statusCode);
      // Same convention as the Go side (otelhttp's default): a 4xx is the
      // client's fault, not a server failure, so only >=500 marks the span
      // Error. (Grafana's built-in Tempo search table doesn't surface span
      // attributes as columns regardless — see accessLogMiddleware below
      // for where method/path/status actually become visible.)
      if (res.statusCode >= 500) {
        span.setStatus({ code: SpanStatusCode.ERROR });
      }
      span.end();
    });
    context.with(ctx, next);
  };
}

/**
 * Express middleware: logs one structured line per request (method/path/
 * status/duration_ms) — matches apps/api's httpx.AccessLog field names
 * exactly, so the "Recent requests" Loki table panel (deploy/grafana/
 * provisioning/dashboards/) can extract the same columns from any service.
 * Mount after tracingMiddleware so trace_id/span_id (via the pino mixin)
 * land on this line too — that's the join key back to the Tempo trace.
 */
export function accessLogMiddleware(log) {
  return (req, res, next) => {
    const start = Date.now();
    res.on('finish', () => {
      log.info(
        { method: req.method, path: req.path, status: res.statusCode, duration_ms: Date.now() - start },
        'http_request',
      );
    });
    next();
  };
}

/** For pino's `mixin` option — stamps trace_id/span_id onto every log line written while a span is active. No-op outside any span. */
export function traceLogMixin() {
  const span = trace.getActiveSpan();
  if (!span) return {};
  const sc = span.spanContext();
  if (!trace.isSpanContextValid(sc)) return {};
  return { trace_id: sc.traceId, span_id: sc.spanId };
}
