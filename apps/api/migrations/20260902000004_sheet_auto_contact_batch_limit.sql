-- +goose Up
ALTER TABLE studio_external_leads_sheet_settings ADD COLUMN auto_contact_batch_limit INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings DROP COLUMN auto_contact_batch_limit;
