-- +goose Up
-- +goose StatementBegin
-- 1. Extend leads table with tracking and status columns
ALTER TABLE leads ADD COLUMN first_name TEXT;
ALTER TABLE leads ADD COLUMN last_name TEXT;
ALTER TABLE leads ADD COLUMN contact_made BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE leads ADD COLUMN hot_lead BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE leads ADD COLUMN trial_purchased BOOLEAN NOT NULL DEFAULT FALSE;

-- Update existing rows to populate first_name and last_name from the combined name
UPDATE leads SET 
    first_name = split_part(name, ' ', 1),
    last_name = substring(name from position(' ' in name) + 1);

-- 2. Create studio google sheets connection settings table
CREATE TABLE studio_sheets_settings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE UNIQUE,
    spreadsheet_id  TEXT NOT NULL,
    tab_name        TEXT NOT NULL DEFAULT 'Leads',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_studio_sheets_settings_studio ON studio_sheets_settings(studio_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS studio_sheets_settings;
ALTER TABLE leads DROP COLUMN IF EXISTS trial_purchased;
ALTER TABLE leads DROP COLUMN IF EXISTS hot_lead;
ALTER TABLE leads DROP COLUMN IF EXISTS contact_made;
ALTER TABLE leads DROP COLUMN IF EXISTS last_name;
ALTER TABLE leads DROP COLUMN IF EXISTS first_name;
-- +goose StatementEnd
