-- +goose Up
-- Tag each knowledge chunk with its business domain (e.g. fitness_gym, yoga).
-- 'all' means the chunk applies to every domain (default).
ALTER TABLE studio_knowledge_chunks
    ADD COLUMN IF NOT EXISTS platform text NOT NULL DEFAULT 'all';

CREATE INDEX IF NOT EXISTS studio_knowledge_chunks_platform_idx
    ON studio_knowledge_chunks (studio_id, platform);

-- +goose Down
DROP INDEX IF EXISTS studio_knowledge_chunks_platform_idx;
ALTER TABLE studio_knowledge_chunks
    DROP COLUMN IF EXISTS platform;
