-- +goose Up
-- +goose StatementBegin
ALTER TABLE leads
ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';

-- Speeds up the "filter by tag" query (l.tags @> ARRAY[$1]) and the
-- distinct-tags-for-a-studio lookup (unnest(tags)) — both scan this
-- column directly.
CREATE INDEX idx_leads_tags ON leads USING GIN (tags);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_leads_tags;

ALTER TABLE leads
DROP COLUMN tags;
-- +goose StatementEnd
