-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS knowledge_base_files JSONB NOT NULL DEFAULT '[]'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS knowledge_base_files;
-- +goose StatementEnd
