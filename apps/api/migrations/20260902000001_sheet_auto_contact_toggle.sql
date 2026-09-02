-- +goose Up
ALTER TABLE studio_external_leads_sheet_settings ADD COLUMN auto_contact_enabled BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings DROP COLUMN auto_contact_enabled;
