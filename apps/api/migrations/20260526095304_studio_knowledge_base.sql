-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS knowledge_base TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS knowledge_base;
-- +goose StatementEnd
