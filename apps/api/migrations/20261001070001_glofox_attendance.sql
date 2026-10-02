-- +goose Up
-- +goose StatementBegin
--
-- Snapshot of Glofox class attendance per member, refreshed every poll by
-- internal/integrations/glofox/attendance.Worker (every 15 min). Not an
-- event log — each poll upserts the latest total count per
-- (studio_id, glofox_user_id), overwriting the previous value. Phone/email
-- are populated only for members who've crossed the attendance threshold
-- (resolving every member's contact details on every poll would mean one
-- Glofox GetMember call per member per poll, which doesn't scale).
--
CREATE TABLE glofox_attendance (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id         UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    glofox_user_id    TEXT NOT NULL,
    name              TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL DEFAULT '',
    email             TEXT NOT NULL DEFAULT '',
    classes_attended  INT NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (studio_id, glofox_user_id)
);
CREATE INDEX idx_glofox_attendance_studio ON glofox_attendance(studio_id);
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS glofox_attendance;
