-- +goose Up
ALTER TABLE studios ADD COLUMN initial_contact_delay_minutes INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE studios DROP COLUMN initial_contact_delay_minutes;
