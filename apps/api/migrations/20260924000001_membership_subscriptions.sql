-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_subscriptions
    ADD COLUMN stripe_subscription_id  TEXT NOT NULL DEFAULT '',
    ADD COLUMN stripe_customer_id      TEXT NOT NULL DEFAULT '',
    ADD COLUMN billing_interval        TEXT NOT NULL DEFAULT 'month',
    ADD COLUMN billing_interval_count  INT  NOT NULL DEFAULT 1,
    ADD COLUMN next_renewal_at         TIMESTAMPTZ,
    ADD COLUMN canceled_at             TIMESTAMPTZ;

CREATE INDEX idx_user_subscriptions_stripe_sub
    ON user_subscriptions (stripe_subscription_id)
    WHERE stripe_subscription_id <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_subscriptions_stripe_sub;

ALTER TABLE user_subscriptions
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS billing_interval,
    DROP COLUMN IF EXISTS billing_interval_count,
    DROP COLUMN IF EXISTS next_renewal_at,
    DROP COLUMN IF EXISTS canceled_at;
-- +goose StatementEnd
