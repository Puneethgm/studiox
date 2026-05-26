-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN IF NOT EXISTS social_planner_enabled BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS social_planner_enabled;
-- +goose StatementEnd
