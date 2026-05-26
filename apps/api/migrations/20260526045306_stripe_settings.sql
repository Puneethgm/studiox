-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN stripe_account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN stripe_secret_key TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN stripe_publishable_key TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN subscription_tier TEXT NOT NULL DEFAULT 'pro';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS subscription_tier;
ALTER TABLE studios DROP COLUMN IF EXISTS stripe_publishable_key;
ALTER TABLE studios DROP COLUMN IF EXISTS stripe_secret_key;
ALTER TABLE studios DROP COLUMN IF EXISTS stripe_account_id;
-- +goose StatementEnd
