-- +goose Up
-- Track when the Glofox first-session WhatsApp trigger was sent for a lead.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS glofox_first_session_notified_at timestamptz;

-- +goose Down
ALTER TABLE leads
    DROP COLUMN IF EXISTS glofox_first_session_notified_at;
