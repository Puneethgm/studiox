-- +goose Up
-- Optional column mappings for the external leads sheet import: "hot lead"
-- (HOT/WARM/COLD) and "trial purchased" (YES/NO) status per row. Used to
-- gate whether a newly imported lead gets auto-contacted.
ALTER TABLE studio_external_leads_sheet_settings
    ADD COLUMN hot_lead_column text NOT NULL DEFAULT '',
    ADD COLUMN trial_purchased_column text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings
    DROP COLUMN hot_lead_column,
    DROP COLUMN trial_purchased_column;
