-- +goose Up
-- Optional date column for the external leads sheet import. When set, the
-- import worker only imports rows whose parsed date falls in the current
-- calendar month, instead of importing the sheet's entire history.
ALTER TABLE studio_external_leads_sheet_settings
    ADD COLUMN date_column text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings
    DROP COLUMN date_column;
