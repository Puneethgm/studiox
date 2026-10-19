-- +goose Up
-- +goose StatementBegin
-- retained_payment_records holds the minimal payment/transaction fields
-- that must be kept for 7 years for financial/tax/accounting/legal
-- purposes (init.md §4.5), copied out of studio_subscriptions and
-- user_subscriptions immediately before a studio is permanently deleted.
-- No FK to studios: this table is intentionally outside the studio's
-- cascade-delete graph so it survives untouched.
CREATE TABLE retained_payment_records (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id               UUID NOT NULL,
    stripe_customer_id      TEXT,
    stripe_subscription_id  TEXT,
    plan_tier               TEXT,
    price_usd               NUMERIC,
    period_start            TIMESTAMPTZ,
    period_end              TIMESTAMPTZ,
    -- Full copy of the source row (studio_subscriptions or
    -- user_subscriptions), beyond the minimal fields promoted into their
    -- own columns above.
    raw_snapshot            JSONB NOT NULL DEFAULT '{}'::jsonb,
    retain_until            TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_retained_payment_records_studio ON retained_payment_records(studio_id);
CREATE INDEX idx_retained_payment_records_retain_until ON retained_payment_records(retain_until);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS retained_payment_records;
-- +goose StatementEnd
