-- +goose Up
-- Per-conversation Do Not Disturb — silences automation for a conversation
-- even when there's no linked lead to carry the older, lead-scoped
-- leads.dnd_enabled flag (e.g. WhatsApp Web contacts imported via history
-- backfill, which deliberately don't get a lead — see
-- HandleInboundWAWebBackfill). ai_worker.go checks both flags.
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS dnd_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN IF EXISTS dnd_enabled;
