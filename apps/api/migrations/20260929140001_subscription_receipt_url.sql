-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS receipt_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS receipt_url;
-- +goose StatementEnd
