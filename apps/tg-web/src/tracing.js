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
import { trace, context } from '@opentelemetry/api';

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
    const span = tracer.startSpan(`${req.method} ${req.path}`);
    const ctx = trace.setSpan(context.active(), span);
    res.on('finish', () => {
      span.setAttribute('http.status_code', res.statusCode);
      span.end();
    });
    context.with(ctx, next);
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
