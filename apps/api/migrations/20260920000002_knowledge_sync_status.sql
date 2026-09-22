-- +goose Up
ALTER TABLE studios ADD COLUMN knowledge_sync_status TEXT NOT NULL DEFAULT 'idle';
ALTER TABLE studios ADD COLUMN knowledge_sync_updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE studios DROP COLUMN knowledge_sync_updated_at;
ALTER TABLE studios DROP COLUMN knowledge_sync_status;
