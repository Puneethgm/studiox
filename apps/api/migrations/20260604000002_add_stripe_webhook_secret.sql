-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN stripe_webhook_secret TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS stripe_webhook_secret;
-- +goose StatementEnd
