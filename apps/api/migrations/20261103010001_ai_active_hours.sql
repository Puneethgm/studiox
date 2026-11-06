-- +goose Up
-- +goose StatementBegin
-- New, separate "AI Active Hours" gate: a daily on/off window (not the
-- existing per-weekday AvailabilitySlots-based business-hours gate) after
-- which the AI stops auto-replying, in a studio-chosen IANA timezone.
-- Disabled by default (ai_active_hours_enabled=false) so no existing
-- studio's behavior changes until a superadmin deliberately opts in.
-- Start/end are stored as TIME (no date), timezone as free text fed
-- straight into Go's time.LoadLocation — same approach already used by
-- studios.availability_timezone, so any valid IANA zone name just works.
ALTER TABLE studios ADD COLUMN ai_active_hours_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE studios ADD COLUMN ai_active_hours_start TIME NOT NULL DEFAULT '00:00';
ALTER TABLE studios ADD COLUMN ai_active_hours_end TIME NOT NULL DEFAULT '23:59';
ALTER TABLE studios ADD COLUMN ai_active_hours_timezone TEXT NOT NULL DEFAULT 'UTC';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS ai_active_hours_timezone;
ALTER TABLE studios DROP COLUMN IF EXISTS ai_active_hours_end;
ALTER TABLE studios DROP COLUMN IF EXISTS ai_active_hours_start;
ALTER TABLE studios DROP COLUMN IF EXISTS ai_active_hours_enabled;
-- +goose StatementEnd
