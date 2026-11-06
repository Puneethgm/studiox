-- +goose Up
-- +goose StatementBegin
-- Discrete reply-tone setting per studio, replacing the previously
-- hardcoded "warm, professional" base-prompt tone (system_rules.tmpl)
-- with a studio-admin-settable choice. DEFAULT 'professional' matches
-- today's actual hardcoded wording exactly, so every existing studio's
-- AI reply behavior is unchanged until an admin deliberately picks a
-- different tone.
ALTER TABLE studios ADD COLUMN reply_tone TEXT NOT NULL DEFAULT 'professional';
ALTER TABLE studios ADD CONSTRAINT studios_reply_tone_check
  CHECK (reply_tone IN ('professional', 'casual', 'friendly'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP CONSTRAINT IF EXISTS studios_reply_tone_check;
ALTER TABLE studios DROP COLUMN IF EXISTS reply_tone;
-- +goose StatementEnd
