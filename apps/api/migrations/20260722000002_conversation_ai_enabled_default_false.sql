-- +goose Up
-- Revert the previous migration: AI auto-reply should only be on for leads
-- that came through the autocontact flow (public_form / external_sheet),
-- which explicitly enables it after creating the conversation. Conversations
-- from any other path (e.g. an unsolicited inbound WhatsApp message from an
-- unknown number) should default to AI off, matching pre-20260721 behavior.
ALTER TABLE conversations
    ALTER COLUMN ai_enabled SET DEFAULT false;

-- +goose Down
ALTER TABLE conversations
    ALTER COLUMN ai_enabled SET DEFAULT true;
