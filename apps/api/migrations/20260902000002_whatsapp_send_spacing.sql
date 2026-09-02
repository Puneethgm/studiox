-- +goose Up
ALTER TABLE studios ADD COLUMN whatsapp_send_spacing_seconds INT NOT NULL DEFAULT 20;

-- +goose Down
ALTER TABLE studios DROP COLUMN whatsapp_send_spacing_seconds;
