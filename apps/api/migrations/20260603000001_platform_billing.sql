-- +goose Up
-- +goose StatementBegin
-- 1. Platform Stripe Settings (Superadmin Config)
CREATE TABLE platform_settings (
    key                 TEXT PRIMARY KEY,
    value               TEXT NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Studio Subscription Record
CREATE TABLE studio_subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id           UUID REFERENCES studios(id) ON DELETE SET NULL,
    stripe_sub_id       TEXT UNIQUE NOT NULL,
    stripe_customer_id  TEXT NOT NULL,
    plan_tier           TEXT NOT NULL, -- 'trial', 'growth', 'pro', 'enterprise'
    status              TEXT NOT NULL, -- 'active', 'past_due', 'canceled'
    price_usd           INT NOT NULL,  -- In cents: 30000, 99900, 129900, 159900
    current_period_end  TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_studio_subscriptions_studio_id ON studio_subscriptions(studio_id);

-- 3. Onboarding Signup Tokens (To prevent studio creation before payment)
CREATE TABLE onboarding_tokens (
    token               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_tier           TEXT NOT NULL,
    stripe_sub_id       TEXT NOT NULL,
    owner_email         TEXT NOT NULL,
    is_used             BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS onboarding_tokens;
DROP TABLE IF EXISTS studio_subscriptions;
DROP TABLE IF EXISTS platform_settings;
-- +goose StatementEnd
