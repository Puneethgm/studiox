-- +goose Up
-- Do Not Disturb: silences all automated messaging (autocontact follow-ups,
-- AI/decision-tree replies) for a lead without changing their pipeline
-- status. Set automatically when a lead replies "stop"/"unsubscribe", or
-- manually from the inbox contact panel.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS dnd_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE leads
    DROP COLUMN IF EXISTS dnd_enabled;
