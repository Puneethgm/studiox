-- +goose Up
-- Tracks the last calendar date a studio's follow-up reminder digest email
-- was sent (FollowupDigestWorker) — guards against sending it twice on the
-- same day if the API process restarts mid-day, since the worker's "due
-- reminders" query (leads.reminder_at <= now()) has no other natural
-- per-day dedup: an unresolved reminder stays due every day until a staff
-- member clears/reschedules it.
ALTER TABLE studios ADD COLUMN followup_digest_last_sent_date DATE;

-- +goose Down
ALTER TABLE studios DROP COLUMN followup_digest_last_sent_date;
