"""Mirrors apps/api's internal/platform/tracing and apps/wa-web|tg-web's
src/tracing.js — same otel-collector -> Tempo -> Grafana pipeline, same
no-op-when-unset contract. Python's OTel SDK uses contextvars for context
propagation, which FastAPI/Starlette's async model supports natively — no
equivalent of the Node side's AsyncHooksContextManager workaround needed,
and FastAPIInstrumentor handles per-request span creation (method, route,
status_code) automatically instead of a hand-rolled middleware.
"""

import json
import logging
import time

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


def init_tracing(service_name: str, otlp_endpoint: str, app) -> None:
    """Empty otlp_endpoint is a deliberate no-op (same contract as the Go/
    Node sides) — local dev without the observability stack running is
    unaffected. Instruments `app` in place."""
    if not otlp_endpoint:
        return

    provider = TracerProvider(resource=Resource.create({SERVICE_NAME: service_name}))
    provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter(endpoint=otlp_endpoint)))
    trace.set_tracer_provider(provider)
    FastAPIInstrumentor.instrument_app(app)


class JSONFormatter(logging.Formatter):
    """Builds the JSON line directly (rather than string-templating) so
    arbitrary extra fields (method/path/status/duration_ms, passed via
    `logger.info(msg, extra={...})`) and trace_id/span_id (read from the
    active span, same mechanism as apps/api's logger.FromCtx / apps/wa-web|
    tg-web's pino mixin) both end up as real JSON keys — not just text
    baked into the message string."""

    # Attributes logging.LogRecord always carries — anything else on the
    # record is a caller-supplied `extra` field, which is what gets merged
    # into the output. Without this list, every record would also dump its
    # own internal bookkeeping (pathname, lineno, etc.) as "extra".
    _STANDARD_ATTRS = set(logging.LogRecord(*([None] * 7), None, None).__dict__.keys())

    def format(self, record: logging.LogRecord) -> str:
        out = {
            "time": self.formatTime(record),
            "level": record.levelname,
            "msg": record.getMessage(),
        }
        for key, value in record.__dict__.items():
            if key not in self._STANDARD_ATTRS and key != "message":
                out[key] = value

        span = trace.get_current_span()
        ctx = span.get_span_context()
        if ctx.is_valid:
            out["trace_id"] = format(ctx.trace_id, "032x")
            out["span_id"] = format(ctx.span_id, "016x")

        return json.dumps(out)


def setup_json_logging() -> logging.Logger:
    logger = logging.getLogger("embeddings")
    logger.setLevel(logging.INFO)
    handler = logging.StreamHandler()
    handler.setFormatter(JSONFormatter())
    logger.addHandler(handler)
    logger.propagate = False
    return logger


class AccessLogMiddleware:
    """Pure ASGI middleware (not Starlette's `@app.middleware("http")` /
    BaseHTTPMiddleware) — that one runs the downstream app in a separate
    anyio task to support streaming responses, which does not reliably
    propagate contextvars back out to code after `call_next()` returns.
    Confirmed live: trace_id was silently missing from every access-log
    line with that approach, despite FastAPIInstrumentor correctly setting
    up the span. A plain ASGI middleware runs in the same task throughout,
    so `trace.get_current_span()` sees the real span at the point we log.

    Logs one structured line per request — method/path/status/duration_ms,
    matching apps/api's httpx.AccessLog / apps/wa-web|tg-web's
    accessLogMiddleware field names exactly, so the "Recent requests" Loki
    table panel extracts the same columns from this service too.
    """

    def __init__(self, app, logger: logging.Logger):
        self.app = app
        self.logger = logger

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            return await self.app(scope, receive, send)

        start = time.monotonic()
        status_holder = {"status": None}

        async def send_wrapper(message):
            if message["type"] == "http.response.start":
                status_holder["status"] = message["status"]
            await send(message)

        await self.app(scope, receive, send_wrapper)

        duration_ms = round((time.monotonic() - start) * 1000, 1)
        self.logger.info(
            "http_request",
            extra={
                "method": scope.get("method"),
                "path": scope.get("path"),
                "status": status_holder["status"],
                "duration_ms": duration_ms,
            },
        )
