-- +goose Up
ALTER TABLE studios ADD COLUMN managed_by_1hero BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE studios DROP COLUMN managed_by_1hero;
