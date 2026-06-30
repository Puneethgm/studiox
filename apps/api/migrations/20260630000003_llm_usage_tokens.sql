-- +goose Up
ALTER TABLE llm_usage_logs ADD COLUMN IF NOT EXISTS tokens_in  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE llm_usage_logs ADD COLUMN IF NOT EXISTS tokens_out INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE llm_usage_logs DROP COLUMN IF EXISTS tokens_in;
ALTER TABLE llm_usage_logs DROP COLUMN IF EXISTS tokens_out;
