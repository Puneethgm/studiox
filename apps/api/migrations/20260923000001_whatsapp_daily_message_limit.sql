-- +goose Up
-- Caps automated/AI-sourced WhatsApp sends per studio per day (Singapore-time
-- calendar day). 0 means unlimited. Default 48 matches the low messaging tier
-- Meta grants unverified WhatsApp Business numbers. Manual agent replies are
-- never affected by this cap.
ALTER TABLE studios ADD COLUMN whatsapp_daily_message_limit INT NOT NULL DEFAULT 48;

-- +goose Down
ALTER TABLE studios DROP COLUMN whatsapp_daily_message_limit;
