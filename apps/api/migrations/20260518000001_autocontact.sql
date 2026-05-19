-- +goose Up
-- +goose StatementBegin
ALTER TABLE leads ADD COLUMN contact_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE leads ADD COLUMN last_contacted_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE leads DROP COLUMN IF EXISTS last_contacted_at;
ALTER TABLE leads DROP COLUMN IF EXISTS contact_attempts;
-- +goose StatementEnd
