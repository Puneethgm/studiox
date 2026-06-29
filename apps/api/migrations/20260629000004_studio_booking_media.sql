-- +goose Up
ALTER TABLE studios
  ADD COLUMN IF NOT EXISTS booking_hero_image_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS booking_hero_video_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studios
  DROP COLUMN IF EXISTS booking_hero_image_url,
  DROP COLUMN IF EXISTS booking_hero_video_url;
