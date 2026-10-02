-- +goose Up
ALTER TABLE glofox_attendance ADD COLUMN plan_limit INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE glofox_attendance DROP COLUMN plan_limit;
