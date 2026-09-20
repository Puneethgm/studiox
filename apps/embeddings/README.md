# apps/embeddings

Local embedding service for the knowledge-base/RAG pipeline — replaces
Gemini's `embedContent` API with a locally-hosted `sentence-transformers`
model, called by the Go API over HTTP.

## Setup

```bash
cd apps/embeddings
python -m venv venv
./venv/bin/pip install -r requirements.txt
```

## Run

```bash
make embeddings   # from repo root — also included in `make dev`
# or directly:
./venv/bin/python main.py
```

First run downloads the model from Hugging Face (can take a couple of
minutes); subsequent runs load from the local cache.

## API

- `POST /embed` — `{"text": "...", "is_query": false}` → `{"embedding": [...], "dim": 768}`.
  `is_query` selects the e5 `"query: "` / `"passage: "` prefix — pass `true`
  when embedding something you're about to search *with*, `false` when
  embedding something you're storing to be searched *for* later.
- `GET /health` — `{"status": "ok", "model": "...", "dim": 768}`.

## Changing the model

`MODEL_NAME` (env var, default `intfloat/multilingual-e5-base`) must stay a
model that outputs **768-dim** vectors — `studio_knowledge_chunks.embedding`
and `messages.embedding` are both `vector(768)` in Postgres. Swapping to a
different-dimension model without also migrating those columns and
re-running the backfill (`apps/api/cmd/backfill-embeddings`) will fail loudly
at write time (pgvector rejects a dimension mismatch on the `::vector` cast)
rather than corrupting data silently — but plan the migration first.
