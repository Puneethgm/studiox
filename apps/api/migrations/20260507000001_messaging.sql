-- +goose Up
-- +goose StatementBegin
--
-- Messaging domain: per-studio multi-channel inbox.
--
-- Phase A (now) uses: channel_accounts, contact_identities, conversations,
-- messages, outbound_jobs.
--
-- Phase D (automations) and E (AI) tables are created here too, with no
-- writers yet — the data model is locked from day one so we don't retrofit
-- a schema migration when those phases land.

-- ── pgvector for future semantic search / RAG over messages ─────────────
-- Created behind IF NOT EXISTS so non-vector environments still migrate.
DO $$
BEGIN
    BEGIN
        CREATE EXTENSION IF NOT EXISTS vector;
    EXCEPTION WHEN OTHERS THEN
        -- Vector extension not available; analyses.embedding column will be
        -- text-typed in that case. We keep going.
        NULL;
    END;
END $$;

-- ── 1. channel_accounts ────────────────────────────────────────────────
-- A studio's connection to one channel (one WhatsApp number, one IG handle, etc.).
-- A studio can have many: e.g., two WhatsApp numbers + an IG handle.
CREATE TABLE channel_accounts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id             UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,

    kind                  TEXT NOT NULL
                          CHECK (kind IN ('whatsapp_meta','instagram_meta','messenger_meta','x_dm')),
    bsp                   TEXT NOT NULL DEFAULT 'meta_direct'
                          CHECK (bsp IN ('meta_direct','twilio')),

    -- Channel-native identifiers. For WhatsApp: WABA id + phone_number_id.
    -- For IG: IG Business id. For Messenger: Page id. For X: account id.
    external_id           TEXT NOT NULL,              -- the per-resource id Meta sends in webhooks
    parent_id             TEXT,                       -- WABA id for WhatsApp; FB Page for IG/Messenger
    display_handle        TEXT NOT NULL,              -- "+65 9xxx xxxx" or "@yogabliss"

    -- Token vault. Encrypted at app level via TOKEN_ENCRYPTION_KEY (AES-256-GCM).
    access_token_enc      TEXT NOT NULL,
    token_expires_at      TIMESTAMPTZ,                -- nullable for non-expiring system user tokens

    -- Lifecycle
    status                TEXT NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active','paused','disconnected','error')),
    last_error            TEXT NOT NULL DEFAULT '',
    connected_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at       TIMESTAMPTZ,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Only live channels are unique so a disconnected account can be reconnected
    -- later as a fresh row without deleting history.
    UNIQUE (kind, external_id) WHERE status <> 'disconnected'
);
CREATE INDEX idx_channel_accounts_studio ON channel_accounts(studio_id);
CREATE INDEX idx_channel_accounts_kind_external ON channel_accounts(kind, external_id);

-- ── 2. contact_identities ──────────────────────────────────────────────
-- Identity stitching: same person across channels → one lead.
-- A WhatsApp message gives us a phone; an IG DM gives us an ig_psid; the same
-- person across both is two identities pointing at one lead.
CREATE TABLE contact_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    lead_id     UUID REFERENCES leads(id) ON DELETE SET NULL,

    kind        TEXT NOT NULL
                CHECK (kind IN ('phone','email','ig_psid','fb_psid','x_id')),
    value       TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (studio_id, kind, value)
);
CREATE INDEX idx_contact_identities_lead ON contact_identities(lead_id);

-- ── 3. conversations ───────────────────────────────────────────────────
-- A thread between a studio and a customer on a single channel.
CREATE TABLE conversations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id             UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    channel_account_id    UUID NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    contact_identity_id   UUID NOT NULL REFERENCES contact_identities(id) ON DELETE RESTRICT,

    -- Channel-native thread/contact identifier (for WhatsApp this is the customer phone)
    external_thread_id    TEXT NOT NULL,

    -- Denormalized for fast joins on lead pages — kept in sync via service layer.
    lead_id               UUID REFERENCES leads(id) ON DELETE SET NULL,

    status                TEXT NOT NULL DEFAULT 'open'
                          CHECK (status IN ('open','snoozed','closed')),
    assigned_to           UUID REFERENCES users(id) ON DELETE SET NULL,
    snoozed_until         TIMESTAMPTZ,

    -- Read tracking (per-conversation; per-user later if multi-agent)
    unread_count          INT NOT NULL DEFAULT 0,
    last_message_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_preview  TEXT NOT NULL DEFAULT '',
    last_message_direction TEXT,                       -- 'inbound' | 'outbound', helps the UI render

    -- AI fields (populated phase E; nullable from day one).
    sentiment_score       NUMERIC(4,3),
    sentiment_trend       TEXT,                        -- 'rising' | 'falling' | 'flat'
    intent_label          TEXT,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (channel_account_id, external_thread_id)
);
CREATE INDEX idx_conversations_studio_open ON conversations(studio_id, last_message_at DESC)
    WHERE status = 'open';
CREATE INDEX idx_conversations_lead ON conversations(lead_id);
CREATE INDEX idx_conversations_studio ON conversations(studio_id, last_message_at DESC);

-- ── 4. messages ────────────────────────────────────────────────────────
CREATE TABLE messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,  -- denormalized for studio-wide listings

    direction       TEXT NOT NULL CHECK (direction IN ('inbound','outbound')),

    -- Who sent this message (covers manual, automation, AI for audit + training).
    source_kind     TEXT NOT NULL DEFAULT 'customer'
                    CHECK (source_kind IN ('customer','studio_user','automation','ai')),
    source_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    source_ref      TEXT,                              -- automation rule_id or ai suggestion id, free-form

    -- Content
    body            TEXT NOT NULL DEFAULT '',
    attachments     JSONB NOT NULL DEFAULT '[]'::jsonb,  -- [{type, url, mime, ...}]

    -- Channel-native identifiers
    external_id     TEXT,                              -- Meta wamid, etc. NULL until first attempt for outbound
    in_reply_to     TEXT,                              -- external_id of the message this replies to

    -- Lifecycle
    status          TEXT NOT NULL DEFAULT 'sent'
                    CHECK (status IN ('pending','sent','delivered','read','failed')),
    failure_reason  TEXT NOT NULL DEFAULT '',

    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at    TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (conversation_id, external_id)              -- NULLs allowed; dedupe webhooks for inbound
);
CREATE INDEX idx_messages_conversation ON messages(conversation_id, sent_at DESC);
CREATE INDEX idx_messages_studio_recent ON messages(studio_id, sent_at DESC);

-- ── 5. outbound_jobs ───────────────────────────────────────────────────
-- Outbox for outbound messages. Manual reply, automation, AI auto-send all
-- enqueue here — one dispatcher path, one place for retries / rate limits /
-- observability.
CREATE TABLE outbound_jobs (
    id               BIGSERIAL PRIMARY KEY,
    studio_id        UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    conversation_id  UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,

    body             TEXT NOT NULL,
    attachments      JSONB NOT NULL DEFAULT '[]'::jsonb,
    template_name    TEXT,                             -- if sending an approved template (>24h window)
    template_params  JSONB,

    source_kind      TEXT NOT NULL CHECK (source_kind IN ('studio_user','automation','ai')),
    source_user_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    source_ref       TEXT,

    scheduled_for    TIMESTAMPTZ NOT NULL DEFAULT now(),
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','sent','failed','dead')),
    attempts         INT NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error       TEXT NOT NULL DEFAULT '',

    -- Once sent, the resulting message id (so the worker can mark message.status = sent).
    message_id       UUID REFERENCES messages(id) ON DELETE SET NULL,

    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at          TIMESTAMPTZ
);
CREATE INDEX idx_outbound_jobs_pickup ON outbound_jobs(status, next_attempt_at)
    WHERE status = 'pending';

-- ── PHASE D stubs (no writers yet; schema is locked) ───────────────────

CREATE TABLE message_templates (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id     UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    body          TEXT NOT NULL,
    channel_kinds TEXT[] NOT NULL DEFAULT '{}',       -- which channels this template is valid on
    -- For WhatsApp templates that need Meta approval, we store the approved name + params.
    whatsapp_template_name  TEXT,
    whatsapp_template_lang  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (studio_id, name)
);

CREATE TABLE automation_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    trigger     JSONB NOT NULL,                        -- {kind: 'message.received', filter: {...}}
    actions     JSONB NOT NULL,                        -- ordered list: [{kind, ...}]
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE automation_runs (
    id              BIGSERIAL PRIMARY KEY,
    rule_id         UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    lead_id         UUID REFERENCES leads(id) ON DELETE SET NULL,
    triggered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    status          TEXT NOT NULL DEFAULT 'success'
                    CHECK (status IN ('success','partial','failed')),
    actions_log     JSONB NOT NULL DEFAULT '[]'::jsonb,
    error           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_automation_runs_studio ON automation_runs(studio_id, triggered_at DESC);

-- ── PHASE E stubs ──────────────────────────────────────────────────────

CREATE TABLE ai_suggestions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    in_reply_to_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,

    draft_body      TEXT NOT NULL,
    model           TEXT NOT NULL,                     -- e.g. 'claude-opus-4-7'
    confidence      NUMERIC(4,3),                      -- 0..1
    rationale       TEXT NOT NULL DEFAULT '',          -- model-supplied reasoning, for audit

    status          TEXT NOT NULL DEFAULT 'proposed'
                    CHECK (status IN ('proposed','accepted','rejected','auto_sent','expired')),
    reviewed_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at     TIMESTAMPTZ,
    sent_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_suggestions_conv ON ai_suggestions(conversation_id, created_at DESC);

CREATE TABLE message_analyses (
    message_id  UUID PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,

    sentiment   NUMERIC(4,3),                          -- -1..1
    intent      TEXT,                                  -- 'pricing' | 'booking' | 'complaint' | ...
    urgency     TEXT,                                  -- 'low' | 'normal' | 'high'
    -- Reserved for future RAG / semantic search. Stored as text array of floats
    -- if pgvector isn't available; service layer handles both shapes.
    embedding_text TEXT,

    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS message_analyses;
DROP TABLE IF EXISTS ai_suggestions;
DROP TABLE IF EXISTS automation_runs;
DROP TABLE IF EXISTS automation_rules;
DROP TABLE IF EXISTS message_templates;
DROP TABLE IF EXISTS outbound_jobs;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS contact_identities;
DROP TABLE IF EXISTS channel_accounts;
-- +goose StatementEnd
