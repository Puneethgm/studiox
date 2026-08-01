-- +goose Up
-- Controls whether the AI keeps auto-replying after the initial greeting
-- for leads imported from a studio's external Google Sheet. When false,
-- only the first greeting message is sent and the conversation is left
-- for manual/human follow-up instead of continuing automatically.
ALTER TABLE studio_external_leads_sheet_settings
    ADD COLUMN continue_ai_after_greeting boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings
    DROP COLUMN continue_ai_after_greeting;
