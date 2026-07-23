-- +goose Up
-- Tracks the lead's active Stripe subscription so a later plan change can
-- cancel the old subscription instead of leaving the customer double-billed
-- on two parallel subscriptions.
ALTER TABLE leads ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT NOT NULL DEFAULT '';
ALTER TABLE leads ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE leads DROP COLUMN IF EXISTS stripe_subscription_id;
ALTER TABLE leads DROP COLUMN IF EXISTS stripe_customer_id;
