-- +goose Up
-- Per-studio opt-in for the Glofox first-session WhatsApp automation. It used
-- to start for whatever studio auto-detection picked as soon as GLOFOX_* env
-- vars existed, and messaged every member who had ever attended. Default OFF;
-- enabled_at is stamped when an admin turns it on and the worker only
-- considers sessions attended after that moment.
ALTER TABLE studios ADD COLUMN glofox_first_session_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE studios ADD COLUMN glofox_first_session_enabled_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE studios DROP COLUMN glofox_first_session_enabled_at;
ALTER TABLE studios DROP COLUMN glofox_first_session_enabled;
