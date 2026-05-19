-- +goose Up
-- +goose StatementBegin
CREATE TABLE campaigns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE RESTRICT,
    slug            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    fitness_plans   TEXT[] NOT NULL DEFAULT '{}',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Slugs must be unique inside a studio. Two studios can both have "spring-promo".
    UNIQUE (studio_id, slug)
);
CREATE INDEX idx_campaigns_studio ON campaigns(studio_id);

CREATE TABLE leads (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE RESTRICT,
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
    name            TEXT NOT NULL,
    email           TEXT NOT NULL,
    phone           TEXT NOT NULL,
    fitness_plan    TEXT NOT NULL,
    goals           TEXT NOT NULL DEFAULT '',
    source          TEXT NOT NULL DEFAULT 'public_form',
    status          TEXT NOT NULL DEFAULT 'new'
                    CHECK (status IN ('new','contacted','trial_booked','member','dropped')),
    notes           TEXT NOT NULL DEFAULT '',
    referrer        TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    ip_address      INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_leads_studio ON leads(studio_id);
CREATE INDEX idx_leads_campaign ON leads(campaign_id);
CREATE INDEX idx_leads_status ON leads(status);
CREATE INDEX idx_leads_created ON leads(created_at DESC);

-- Outbox: every lead write atomically inserts an outbox row. The sheets worker
-- drains it. Same idea as before, just with studio context in the payload.
CREATE TABLE outbox (
    id                  BIGSERIAL PRIMARY KEY,
    aggregate_type      TEXT NOT NULL,
    aggregate_id        UUID NOT NULL,
    event_type          TEXT NOT NULL,
    destination         TEXT NOT NULL,
    payload             JSONB NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','sent','failed','dead')),
    attempts            INT NOT NULL DEFAULT 0,
    next_attempt_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at             TIMESTAMPTZ
);
CREATE INDEX idx_outbox_pickup ON outbox(status, next_attempt_at) WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS leads;
DROP TABLE IF EXISTS campaigns;
-- +goose StatementEnd
