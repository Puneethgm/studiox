-- +goose Up
ALTER TABLE glofox_attendance ADD COLUMN plan_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE glofox_attendance DROP COLUMN plan_name;
