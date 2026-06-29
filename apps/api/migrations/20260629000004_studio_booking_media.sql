ALTER TABLE studios
  ADD COLUMN IF NOT EXISTS booking_hero_image_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS booking_hero_video_url TEXT NOT NULL DEFAULT '';
