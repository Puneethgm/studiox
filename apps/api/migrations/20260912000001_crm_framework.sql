-- +goose Up
--
-- Generic CRM integration framework — replaces the hardcoded global Glofox
-- client (GLOFOX_API_KEY/TOKEN/BRANCH_ID in .env) with a data-driven system:
-- a super-admin onboards a CRM's shape once (crm_providers/crm_operations),
-- then connects it per studio with that studio's own credentials
-- (crm_connections). See internal/integrations/crm.
--
-- ── crm_providers ────────────────────────────────────────────────────────
-- One row per onboarded CRM (Glofox, Mindbody, ...). Platform-wide catalog —
-- the *shape* of a CRM's API isn't studio-scoped, only its credentials are.
CREATE TABLE crm_providers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',

    base_url          TEXT NOT NULL DEFAULT '',  -- e.g. "https://gf-api.aws.glofox.com/prod"
    auth_type         TEXT NOT NULL
                      CHECK (auth_type IN ('bearer','api_key','basic')),
    -- [{"key":"api_key","label":"API Key","secret":true}, ...] — drives both
    -- the dynamic "connect a studio" form and how the executor applies auth.
    auth_field_defs   JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Raw uploaded doc (OpenAPI/Swagger/free text), kept for audit/re-parsing.
    spec_source       TEXT NOT NULL DEFAULT '',

    status            TEXT NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft','active')),

    created_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── crm_operations ───────────────────────────────────────────────────────
-- One row per (provider, operation). operation_key is one of the fixed
-- catalog values in internal/integrations/crm/operations.go, mirroring what
-- glofox.Client already implements today (create_lead, register_user,
-- purchase_membership, find_membership_plan, get_member, list_bookings).
CREATE TABLE crm_operations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    crm_provider_id   UUID NOT NULL REFERENCES crm_providers(id) ON DELETE CASCADE,

    operation_key     TEXT NOT NULL,
    http_method       TEXT NOT NULL,
    path_template     TEXT NOT NULL DEFAULT '',       -- e.g. "/v2/members/{member_id}"
    request_mapping   JSONB NOT NULL DEFAULT '{}'::jsonb,  -- operation params -> path/query/body
    response_mapping  JSONB NOT NULL DEFAULT '{}'::jsonb,  -- response body -> fields we need

    reviewed          BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (crm_provider_id, operation_key)
);

-- ── crm_connections ──────────────────────────────────────────────────────
-- Per-studio connection: which provider a studio uses + its own credentials.
CREATE TABLE crm_connections (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id         UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    crm_provider_id   UUID NOT NULL REFERENCES crm_providers(id) ON DELETE CASCADE,

    -- AES-256-GCM via internal/platform/secrets.Cipher, same mechanism as
    -- channel_accounts.access_token_enc. Plaintext is a JSON blob of
    -- {field_key: value} for whatever the provider's auth_field_defs need.
    credentials_enc   TEXT NOT NULL,

    status            TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active','disconnected')),

    connected_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (studio_id, crm_provider_id)
);

CREATE INDEX idx_crm_connections_studio ON crm_connections(studio_id);

-- ── ai_task_configs ──────────────────────────────────────────────────────
-- Not CRM-specific: which LLM handles which AI-driven admin task, starting
-- with 'crm_doc_parsing'. A NULL api_key_enc means "use that provider's
-- existing platform-level .env key" rather than requiring a duplicate.
CREATE TABLE ai_task_configs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose           TEXT NOT NULL UNIQUE,
    provider          TEXT NOT NULL
                      CHECK (provider IN ('claude','gemini','groq')),
    model             TEXT NOT NULL,
    api_key_enc       TEXT,

    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        UUID REFERENCES users(id)
);

-- +goose Down
DROP TABLE ai_task_configs;
DROP TABLE crm_connections;
DROP TABLE crm_operations;
DROP TABLE crm_providers;
