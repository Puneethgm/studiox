-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS gemini_api_key TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS gemini_api_key;
-- +goose StatementEnd
