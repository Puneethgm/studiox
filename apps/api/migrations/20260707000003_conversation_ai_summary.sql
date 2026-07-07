-- +goose Up

ALTER TABLE conversations
    ADD COLUMN ai_context_summary TEXT NOT NULL DEFAULT '',
    ADD COLUMN ai_context_summary_updated_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE conversations
    DROP COLUMN IF EXISTS ai_context_summary,
    DROP COLUMN IF EXISTS ai_context_summary_updated_at;
