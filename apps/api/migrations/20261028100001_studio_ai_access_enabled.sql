-- +goose Up
-- Superadmin-only master switch for whether a studio gets AI replies at
-- all — distinct from conversations.ai_enabled (per-conversation, staff-
-- togglable). Defaults true so no existing studio loses AI access the
-- moment this ships; a superadmin opts a studio OUT, not back in.
ALTER TABLE studios ADD COLUMN IF NOT EXISTS ai_access_enabled BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE studios DROP COLUMN IF EXISTS ai_access_enabled;
