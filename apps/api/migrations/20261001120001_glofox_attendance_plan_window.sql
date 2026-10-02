-- +goose Up
ALTER TABLE glofox_attendance ADD COLUMN plan_start TIMESTAMPTZ;
ALTER TABLE glofox_attendance ADD COLUMN plan_end TIMESTAMPTZ;

-- +goose Down
ALTER TABLE glofox_attendance DROP COLUMN plan_start;
ALTER TABLE glofox_attendance DROP COLUMN plan_end;
