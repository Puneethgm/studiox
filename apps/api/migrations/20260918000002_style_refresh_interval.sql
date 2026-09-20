-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS style_refresh_interval_minutes INT NOT NULL DEFAULT 240;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS style_refresh_interval_minutes;
-- +goose StatementEnd
