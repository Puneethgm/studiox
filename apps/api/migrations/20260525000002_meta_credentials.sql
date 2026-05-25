-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN meta_app_id TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN meta_app_secret TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS meta_app_secret;
ALTER TABLE studios DROP COLUMN IF EXISTS meta_app_id;
-- +goose StatementEnd
