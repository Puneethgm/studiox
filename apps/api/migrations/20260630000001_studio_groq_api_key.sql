-- +goose Up
ALTER TABLE studios ADD COLUMN IF NOT EXISTS groq_api_key TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studios DROP COLUMN IF EXISTS groq_api_key;
