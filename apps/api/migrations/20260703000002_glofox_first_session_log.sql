-- +goose Up
-- Tracks which Glofox members have already received a first-session WhatsApp.
-- Keyed by (glofox_user_id, branch_id) so it works even when the member has
-- no corresponding StudioX lead row.
CREATE TABLE IF NOT EXISTS glofox_first_session_log (
    glofox_user_id text        NOT NULL,
    branch_id      text        NOT NULL,
    studio_id      uuid        NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    notified_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (glofox_user_id, branch_id)
);

-- +goose Down
DROP TABLE IF EXISTS glofox_first_session_log;
