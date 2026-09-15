-- +goose Up
-- +goose StatementBegin
-- staff_reply_count_total: cheap running counter of source_kind='studio_user'
-- messages ever sent for this studio, bumped once per staff-authored outbound
-- send. Compared against style_profile_source_count to decide when the style
-- worker has enough new material to rebuild the profile, without a COUNT(*)
-- scan over messages on every poll.
ALTER TABLE studios ADD COLUMN IF NOT EXISTS staff_reply_count_total INT NOT NULL DEFAULT 0;

-- communication_style_profile: a short, studio-visible, LLM-generated writeup
-- of how this studio's staff actually talk to customers (tone, phrasing,
-- emoji use, how they handle pricing/objections) — distilled from their own
-- source_kind='studio_user' message history. Editable by the studio admin,
-- same as the rest of the Knowledge Base page.
ALTER TABLE studios ADD COLUMN IF NOT EXISTS communication_style_profile TEXT NOT NULL DEFAULT '';

-- style_profile_source_count: value of staff_reply_count_total at the time
-- the profile was last (re)built. The style worker rebuilds once the gap
-- between the two exceeds its threshold.
ALTER TABLE studios ADD COLUMN IF NOT EXISTS style_profile_source_count INT NOT NULL DEFAULT 0;

ALTER TABLE studios ADD COLUMN IF NOT EXISTS style_profile_updated_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS staff_reply_count_total;
ALTER TABLE studios DROP COLUMN IF EXISTS communication_style_profile;
ALTER TABLE studios DROP COLUMN IF EXISTS style_profile_source_count;
ALTER TABLE studios DROP COLUMN IF EXISTS style_profile_updated_at;
-- +goose StatementEnd
