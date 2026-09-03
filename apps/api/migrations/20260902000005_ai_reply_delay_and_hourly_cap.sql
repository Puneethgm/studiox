-- +goose Up
ALTER TABLE studios ADD COLUMN ai_reply_delay_seconds INT NOT NULL DEFAULT 0;
ALTER TABLE studios ADD COLUMN whatsapp_hourly_send_limit INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE studios DROP COLUMN ai_reply_delay_seconds;
ALTER TABLE studios DROP COLUMN whatsapp_hourly_send_limit;
