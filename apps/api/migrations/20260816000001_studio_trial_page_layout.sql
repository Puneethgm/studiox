-- +goose Up
ALTER TABLE studios ADD COLUMN trial_page_layout JSONB;

-- +goose Down
ALTER TABLE studios DROP COLUMN trial_page_layout;
