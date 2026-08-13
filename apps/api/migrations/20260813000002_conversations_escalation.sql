-- +goose Up
ALTER TABLE conversations ADD COLUMN escalated_at TIMESTAMPTZ;
ALTER TABLE conversations ADD COLUMN escalated_reason TEXT;

-- +goose Down
ALTER TABLE conversations DROP COLUMN escalated_reason;
ALTER TABLE conversations DROP COLUMN escalated_at;
