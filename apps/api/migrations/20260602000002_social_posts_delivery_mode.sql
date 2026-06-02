-- +goose Up
-- +goose StatementBegin
ALTER TABLE social_posts
    ADD COLUMN delivery_mode TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN external_resource_name TEXT NOT NULL DEFAULT '';

ALTER TABLE social_posts
    ADD CONSTRAINT social_posts_delivery_mode_check
    CHECK (delivery_mode IN ('unknown', 'mock', 'live'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE social_posts DROP CONSTRAINT IF EXISTS social_posts_delivery_mode_check;
ALTER TABLE social_posts
    DROP COLUMN IF EXISTS external_resource_name,
    DROP COLUMN IF EXISTS delivery_mode;
-- +goose StatementEnd
