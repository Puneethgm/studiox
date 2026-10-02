-- +goose Up
-- +goose StatementBegin
--
-- WhatsApp broadcast campaigns: a studio uploads a standalone contact list
-- (not leads — no pipeline/campaign entanglement, per internal/messaging
-- "Manual Actions" bulk-send feature), composes one message, and schedules
-- it to the whole list. Delivery itself reuses the existing outbound_jobs
-- queue and SourceAutomation path — these tables only track the
-- list/campaign/recipient bookkeeping and resume state needed to respect
-- the studio's daily WhatsApp send limit across multiple days.
--

CREATE TABLE broadcast_lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_broadcast_lists_studio ON broadcast_lists(studio_id);

CREATE TABLE broadcast_contacts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    broadcast_list_id UUID NOT NULL REFERENCES broadcast_lists(id) ON DELETE CASCADE,
    name              TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL, -- always stored with a leading '+' — see broadcast import's country-code step
    email             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_broadcast_contacts_list ON broadcast_contacts(broadcast_list_id);

-- Ephemeral: holds a parsed-but-not-yet-confirmed import between "upload
-- the file" and "confirm with a default country code for numbers missing
-- one" — a second request/response round trip, so the file itself never
-- needs to be re-uploaded. Rows are deleted once confirmed; a stale row
-- left behind by an abandoned import is harmless and small, not worth a
-- cleanup job at this scale.
CREATE TABLE broadcast_import_staging (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    rows        JSONB NOT NULL, -- [{"name":"","phone":"","email":""}, ...]
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_broadcast_import_staging_studio ON broadcast_import_staging(studio_id);

CREATE TABLE broadcast_campaigns (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id         UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    broadcast_list_id UUID NOT NULL REFERENCES broadcast_lists(id) ON DELETE CASCADE,
    body              TEXT NOT NULL,
    attachments       JSONB NOT NULL DEFAULT '[]'::jsonb,
    scheduled_for     TIMESTAMPTZ NOT NULL,
    status            TEXT NOT NULL DEFAULT 'scheduled'
                      CHECK (status IN ('scheduled','sending','completed','canceled')),
    total_count       INT NOT NULL DEFAULT 0,
    enqueued_count    INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_broadcast_campaigns_studio ON broadcast_campaigns(studio_id);
CREATE INDEX idx_broadcast_campaigns_active ON broadcast_campaigns(status) WHERE status IN ('scheduled','sending');

-- One row per contact in the campaign's list, at creation time (a snapshot
-- of list membership — adding to the list later doesn't retroactively grow
-- a campaign already scheduled against it). status tracks enqueue into
-- outbound_jobs, not delivery — delivery success/failure is already
-- tracked on outbound_jobs itself via the existing outbound worker.
CREATE TABLE broadcast_campaign_recipients (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id          UUID NOT NULL REFERENCES broadcast_campaigns(id) ON DELETE CASCADE,
    broadcast_contact_id UUID NOT NULL REFERENCES broadcast_contacts(id) ON DELETE CASCADE,
    status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','enqueued','failed')),
    outbound_job_id      BIGINT REFERENCES outbound_jobs(id) ON DELETE SET NULL,
    enqueued_at          TIMESTAMPTZ,
    last_error           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_broadcast_campaign_recipients_campaign ON broadcast_campaign_recipients(campaign_id);
CREATE INDEX idx_broadcast_campaign_recipients_pending ON broadcast_campaign_recipients(campaign_id)
    WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS broadcast_campaign_recipients;
DROP TABLE IF EXISTS broadcast_campaigns;
DROP TABLE IF EXISTS broadcast_import_staging;
DROP TABLE IF EXISTS broadcast_contacts;
DROP TABLE IF EXISTS broadcast_lists;
