-- +goose Up

-- HNSW index for fast approximate nearest-neighbour search on knowledge chunks
-- Replaces sequential scan; cosine distance matches the <=> operator used in queries.
CREATE INDEX IF NOT EXISTS idx_studio_knowledge_chunks_embedding_hnsw
    ON studio_knowledge_chunks
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Full-text search index for BM25 hybrid retrieval
ALTER TABLE studio_knowledge_chunks
    ADD COLUMN IF NOT EXISTS content_tsv tsvector
        GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

CREATE INDEX IF NOT EXISTS idx_studio_knowledge_chunks_fts
    ON studio_knowledge_chunks
    USING gin(content_tsv);

-- Store pre-computed embeddings on messages for semantic history lookup
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS embedding vector(768);

CREATE INDEX IF NOT EXISTS idx_messages_embedding_hnsw
    ON messages
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Store LLM-classified intent and sentiment (replaces keyword matching)
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS intent TEXT,      -- e.g. pricing_question, booking_inquiry, objection, ready_to_buy, off_topic
    ADD COLUMN IF NOT EXISTS sentiment SMALLINT, -- -1, 0, 1
    ADD COLUMN IF NOT EXISTS sentiment_confidence REAL;

-- +goose Down
DROP INDEX IF EXISTS idx_studio_knowledge_chunks_embedding_hnsw;
DROP INDEX IF EXISTS idx_studio_knowledge_chunks_fts;
DROP INDEX IF EXISTS idx_messages_embedding_hnsw;
ALTER TABLE studio_knowledge_chunks DROP COLUMN IF EXISTS content_tsv;
ALTER TABLE messages DROP COLUMN IF EXISTS embedding;
ALTER TABLE messages DROP COLUMN IF EXISTS intent;
ALTER TABLE messages DROP COLUMN IF EXISTS sentiment;
ALTER TABLE messages DROP COLUMN IF EXISTS sentiment_confidence;
