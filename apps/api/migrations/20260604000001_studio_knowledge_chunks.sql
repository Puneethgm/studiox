-- +goose Up
-- Enable pgvector extension (available via pgvector/pgvector:pg16 image)
CREATE EXTENSION IF NOT EXISTS vector;

-- Chunked knowledge base content per studio with vector embeddings
CREATE TABLE studio_knowledge_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,   -- 'text' | 'file'
    source_name TEXT NOT NULL,   -- 'knowledge_base' or file name
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768),       -- Gemini text-embedding-004 = 768 dims
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Strict per-studio index for multi-tenant isolation + fast retrieval
CREATE INDEX idx_studio_knowledge_chunks_studio_id
    ON studio_knowledge_chunks(studio_id);

-- +goose Down
DROP TABLE IF EXISTS studio_knowledge_chunks;
DROP EXTENSION IF EXISTS vector;
