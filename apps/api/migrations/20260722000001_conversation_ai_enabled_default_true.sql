-- +goose Up
-- New conversations should default to AI auto-reply ON. Conversations created
-- outside the autocontact flow (which explicitly enables it after creation)
-- were otherwise silently defaulting to OFF.
ALTER TABLE conversations
    ALTER COLUMN ai_enabled SET DEFAULT true;

-- +goose Down
ALTER TABLE conversations
    ALTER COLUMN ai_enabled SET DEFAULT false;
