-- +goose Up
-- Reverting whatsapp_hourly_send_limit from 20260902000005 — the studio
-- wants a single lead-to-lead interval (whatsapp_send_spacing_seconds),
-- not a separate per-hour bucket concept. Keeping ai_reply_delay_seconds
-- from that same migration.
ALTER TABLE studios DROP COLUMN whatsapp_hourly_send_limit;

-- +goose Down
ALTER TABLE studios ADD COLUMN whatsapp_hourly_send_limit INT NOT NULL DEFAULT 0;
