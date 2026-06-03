-- +goose Up
-- Enable pgvector extension if available
CREATE EXTENSION IF NOT EXISTS vector;

-- Table to store chunked studio knowledge base content with vector embeddings
CREATE TABLE studio_knowledge_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL, -- 'text' or 'file'
    source_name TEXT NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768), -- Gemini text-embedding-004 dimensions (768)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index on studio_id for strict multi-tenant isolation and fast lookup
CREATE INDEX idx_studio_knowledge_chunks_studio_id ON studio_knowledge_chunks(studio_id);

-- +goose Down
DROP TABLE IF EXISTS studio_knowledge_chunks;
DROP EXTENSION IF EXISTS vector;
