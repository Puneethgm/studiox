-- +goose Up
-- +goose StatementBegin
ALTER TABLE social_posts
    ADD COLUMN campaign_share_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE social_posts
    DROP COLUMN IF EXISTS campaign_share_url;
-- +goose StatementEnd
