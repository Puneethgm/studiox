-- +goose Up
-- +goose StatementBegin
CREATE TABLE social_posts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id    UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    campaign     TEXT NOT NULL,
    platform     TEXT NOT NULL,
    copy         TEXT NOT NULL,
    media_url    TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL CHECK (status IN ('draft', 'scheduled', 'published', 'failed')),
    scheduled_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_social_posts_studio ON social_posts(studio_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS social_posts;
-- +goose StatementEnd
