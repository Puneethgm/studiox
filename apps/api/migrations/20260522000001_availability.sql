-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS availability_slots JSONB DEFAULT '[]'::jsonb;
ALTER TABLE studios ADD COLUMN IF NOT EXISTS availability_timezone TEXT NOT NULL DEFAULT 'Asia/Kolkata';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS availability_slots;
ALTER TABLE studios DROP COLUMN IF EXISTS availability_timezone;
-- +goose StatementEnd
