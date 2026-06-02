-- +goose Up
-- +goose StatementBegin
CREATE TABLE plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    plan_name       TEXT NOT NULL,
    price_sgd       INT NOT NULL DEFAULT 0,
    billing_cycle   TEXT NOT NULL DEFAULT 'monthly',
    features        TEXT[] NOT NULL DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_plans_studio ON plans(studio_id);

CREATE TABLE user_subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id           UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    lead_id             UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    plan_id             UUID NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    plan_name           TEXT NOT NULL,
    amount_paid         INT NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'SGD',
    payment_id          TEXT NOT NULL DEFAULT '',
    payment_status      TEXT NOT NULL DEFAULT 'pending',
    subscription_status TEXT NOT NULL DEFAULT 'active',
    start_date          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expiry_date         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_subscriptions_studio ON user_subscriptions(studio_id);
CREATE INDEX idx_user_subscriptions_lead ON user_subscriptions(lead_id);
CREATE INDEX idx_user_subscriptions_plan ON user_subscriptions(plan_id);

-- Drop unused currency columns from studios to enforce SGD
ALTER TABLE studios DROP COLUMN IF EXISTS trial_amount_inr;
ALTER TABLE studios DROP COLUMN IF EXISTS trial_amount_usd;

-- Add default plans to existing studios
INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features)
SELECT id, 'Trial', 0, 'one_time', ARRAY['Free', 'Limited features', 'Valid for 7 days']
FROM studios;

INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features)
SELECT id, 'Basic', 2900, 'monthly', ARRAY['Feature A', 'Feature B', 'Feature C']
FROM studios;

INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features)
SELECT id, 'Pro', 9900, 'monthly', ARRAY['All Basic Features', 'Feature D', 'Feature E', 'Studio Plan']
FROM studios;

INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features)
SELECT id, 'Pro Plus', 19900, 'monthly', ARRAY['All Pro Features', 'Feature F', 'Feature G', 'Studio Plan']
FROM studios;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN trial_amount_inr INT NOT NULL DEFAULT 0;
ALTER TABLE studios ADD COLUMN trial_amount_usd INT NOT NULL DEFAULT 0;

DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS plans;
-- +goose StatementEnd
