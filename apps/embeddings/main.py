"""Local embedding service for Project-X's knowledge-base/RAG pipeline.

Replaces the Gemini embedContent API with a locally-hosted sentence-transformers
model (default: intfloat/multilingual-e5-base, 768-dim — matches the existing
pgvector column width, so no DB migration is needed).

Run with `make embeddings` from the repo root, or directly:
    ./venv/bin/python main.py
"""

import os

from contextlib import asynccontextmanager

from dotenv import load_dotenv
from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

from tracing import AccessLogMiddleware, init_tracing, setup_json_logging

load_dotenv()

MODEL_NAME = os.environ.get("MODEL_NAME", "intfloat/multilingual-e5-base")
PORT = int(os.environ.get("PORT", "8001"))

model: SentenceTransformer | None = None
log = setup_json_logging()


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Load once at startup, not per-request — the model is ~1GB+ in memory
    # and loading it per-request would be both slow and wasteful. Run this
    # service with a single uvicorn worker so the model isn't loaded twice.
    global model
    model = SentenceTransformer(MODEL_NAME)
    yield


app = FastAPI(lifespan=lifespan)

# Starlette wraps middleware so the *last* one added ends up outermost —
# AccessLogMiddleware must be added before init_tracing's instrument_app()
# call, so the OTel span (added after, thus outermost) stays active for the
# whole time this middleware's code runs. Added the other way around first
# and confirmed live that trace_id was then silently missing from every
# access-log line, despite the span itself reaching Tempo just fine.
app.add_middleware(AccessLogMiddleware, logger=log)

# Empty OTEL_EXPORTER_OTLP_ENDPOINT is a deliberate no-op (see tracing.py) —
# local dev without the observability stack running is unaffected. Mirrors
# apps/api / apps/wa-web|tg-web's own tracing setup.
init_tracing("embeddings", os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", ""), app)


class EmbedRequest(BaseModel):
    text: str
    is_query: bool = False


class EmbedResponse(BaseModel):
    embedding: list[float]
    dim: int


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest) -> EmbedResponse:
    # e5 models need a "query: " / "passage: " prefix for good retrieval
    # quality. This happens here, server-side, so every Go call site only
    # ever passes a boolean and never has to build the prefixed string
    # itself — that's the single place a query/passage mixup could happen.
    prefix = "query: " if req.is_query else "passage: "
    vec = model.encode(prefix + req.text, normalize_embeddings=True)
    return EmbedResponse(embedding=vec.tolist(), dim=len(vec))


@app.get("/health")
def health() -> dict:
    dim = model.get_sentence_embedding_dimension() if model else None
    return {"status": "ok", "model": MODEL_NAME, "dim": dim}


# No auth for v1 — this service is local-dev-only, reachable only from the Go
# API on localhost. If it's ever exposed beyond a single machine (containerized
# alongside the API, for example), adopt the same shared `x-internal-key`
# header pattern apps/wa-web and apps/tg-web use (checked against an
# INTERNAL_API_KEY env var) rather than leaving this open.

# No batch endpoint for v1 either — every current Go call site embeds one
# text at a time (either live/latency-sensitive, or already sequential with
# a rate-limit sleep left over from the Gemini days), so batching wouldn't
# change throughput today. Add a POST /embed_batch here first if KB sync for
# a very large studio turns out to be too slow.

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=PORT, workers=1)
