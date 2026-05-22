-- +goose Up
-- +goose StatementBegin
ALTER TABLE leads ADD COLUMN auto_contact_stage TEXT NOT NULL DEFAULT 'initial';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE leads DROP COLUMN IF EXISTS auto_contact_stage;
-- +goose StatementEnd
