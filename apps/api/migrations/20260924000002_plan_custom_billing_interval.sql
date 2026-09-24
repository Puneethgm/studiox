-- +goose Up
-- +goose StatementBegin
-- Lets a plan's billing_cycle be 'custom', backed by an explicit Stripe
-- interval unit + count (e.g. "every 5 weeks") instead of one of the fixed
-- preset labels. Ignored for preset cycles — those still derive their
-- interval/count from billing_cycle via billing.RecurringForCycle.
ALTER TABLE plans
    ADD COLUMN billing_interval       TEXT NOT NULL DEFAULT '',
    ADD COLUMN billing_interval_count INT  NOT NULL DEFAULT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE plans
    DROP COLUMN IF EXISTS billing_interval,
    DROP COLUMN IF EXISTS billing_interval_count;
-- +goose StatementEnd
