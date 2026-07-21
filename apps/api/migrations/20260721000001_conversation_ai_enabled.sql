-- +goose Up
-- Per-conversation AI auto-reply toggle.
-- Defaults to false so existing/incoming contacts don't get auto-replied
-- until a studio user explicitly enables it in the inbox.
-- Conversations created by the autocontact worker (form leads) have this
-- set to true immediately after creation.
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS ai_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN IF EXISTS ai_enabled;
