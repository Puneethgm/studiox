-- +goose Up
-- The AI reply "staff active" gate (apps/api/internal/messaging/gates.go)
-- previously used a hardcoded 10-minute window for every studio: if staff
-- sent a message in a conversation within that window, the AI stayed
-- silent. Made configurable per studio so it isn't a one-size-fits-all
-- constant — same pattern as ai_reply_delay_seconds.
ALTER TABLE studios ADD COLUMN staff_active_window_minutes INT NOT NULL DEFAULT 10;

-- +goose Down
ALTER TABLE studios DROP COLUMN staff_active_window_minutes;
