-- +goose Up
-- Replaces the once-a-day, whole-studio digest dedup
-- (studios.followup_digest_last_sent_date, see 20261105010001) with a
-- per-lead marker: FollowupDigestWorker now warns 10 minutes BEFORE a
-- reminder's reminder_at, not once a day for everything already overdue, so
-- "already sent for this specific reminder" has to be tracked per lead, not
-- per studio per day. NULL means "not yet notified for the current
-- reminder_at value" — cleared back to NULL by UpdateLead whenever
-- reminder_at changes, so editing a reminder's time always re-arms it.
ALTER TABLE leads ADD COLUMN reminder_notified_at TIMESTAMPTZ;

ALTER TABLE studios DROP COLUMN followup_digest_last_sent_date;

-- +goose Down
ALTER TABLE studios ADD COLUMN followup_digest_last_sent_date DATE;

ALTER TABLE leads DROP COLUMN reminder_notified_at;
