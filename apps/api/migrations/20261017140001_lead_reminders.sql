-- +goose Up
-- +goose StatementBegin
-- Manual, staff-set follow-up reminders — distinct from needs_manual_followup
-- (an AI/system-owned escalation flag set automatically on certain trial-
-- booking paths, cleared automatically once a human replies; see
-- internal/messaging's SetLeadNeedsManualFollowup). This is purely
-- human-driven: a studio user picks a date/time to be reminded to follow
-- up with a specific lead, with an optional note on what to say.
ALTER TABLE leads
ADD COLUMN reminder_at TIMESTAMPTZ,
ADD COLUMN reminder_note TEXT NOT NULL DEFAULT '';

-- Speeds up "which leads have a reminder due" (reminder_at <= now()),
-- both the dashboard stat card's count query and the Leads tab's
-- reminderDue filter. Partial index since most leads will never have one.
CREATE INDEX idx_leads_reminder_at ON leads (reminder_at) WHERE reminder_at IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_leads_reminder_at;

ALTER TABLE leads
DROP COLUMN reminder_at,
DROP COLUMN reminder_note;
-- +goose StatementEnd
