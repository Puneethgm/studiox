-- +goose Up
ALTER TABLE conversations ADD COLUMN is_starred BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE conversations DROP COLUMN is_starred;
