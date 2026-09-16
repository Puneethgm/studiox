-- +goose Up
-- Instagram feed/Reel captions can never contain a clickable link (a Meta
-- platform restriction, not something any app can change) — the only way to
-- attach a genuinely clickable link to a specific published post is an
-- Instagram Story with a Link Sticker, which Meta's Content Publishing API
-- explicitly does not support adding programmatically. So for a post with a
-- campaign link, staff are shown an "action needed" prompt to post it as a
-- Story with the link sticker themselves; this column tracks when they did.
ALTER TABLE social_posts ADD COLUMN story_link_posted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE social_posts DROP COLUMN story_link_posted_at;
