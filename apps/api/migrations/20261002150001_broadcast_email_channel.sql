-- +goose Up
-- +goose StatementBegin
--
-- Broadcasts can now go out over the studio's connected email channel
-- (kind='email_smtp') instead of only WhatsApp. channel_kind records which
-- one a campaign was sent on; subject is required for email (WhatsApp has
-- no concept of one, so it's left '' there). outbound_jobs.subject is the
-- generic companion — any future email_smtp send through the shared
-- outbound queue needs somewhere to carry it, not just broadcasts.
--

ALTER TABLE outbound_jobs ADD COLUMN subject TEXT NOT NULL DEFAULT '';

ALTER TABLE broadcast_campaigns ADD COLUMN channel_kind TEXT NOT NULL DEFAULT 'whatsapp'
    CHECK (channel_kind IN ('whatsapp', 'email'));
ALTER TABLE broadcast_campaigns ADD COLUMN subject TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE broadcast_campaigns DROP COLUMN subject;
ALTER TABLE broadcast_campaigns DROP COLUMN channel_kind;
ALTER TABLE outbound_jobs DROP COLUMN subject;
-- +goose StatementEnd
