-- +goose Up
-- +goose StatementBegin
--
-- Per-studio "classes attended" qualifying threshold for the Attendance
-- page/worker (internal/integrations/glofox/attendance) — studio-admin
-- configurable via the UI instead of hardcoded. One row per studio;
-- absence of a row means "use the default" (5), enforced in application
-- code, not here.
--
CREATE TABLE glofox_attendance_settings (
    studio_id            UUID PRIMARY KEY REFERENCES studios(id) ON DELETE CASCADE,
    qualifying_threshold INT NOT NULL DEFAULT 5 CHECK (qualifying_threshold > 0),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS glofox_attendance_settings;
