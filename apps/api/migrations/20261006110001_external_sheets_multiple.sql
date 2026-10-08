-- +goose Up
-- A studio can import leads from several external Google Sheets (or several tabs of one),
-- each with its own column mapping and auto-contact options. It used to be one row per
-- studio. The same spreadsheet+tab can't be added twice to one studio.
ALTER TABLE studio_external_leads_sheet_settings
    DROP CONSTRAINT studio_external_leads_sheet_settings_studio_id_key;
ALTER TABLE studio_external_leads_sheet_settings
    ADD CONSTRAINT studio_external_leads_sheet_settings_sheet_key UNIQUE (studio_id, spreadsheet_id, tab_name);

-- The import progress (last row imported) is per sheet+tab AND studio now, so two studios
-- importing the same sheet no longer share, and corrupt, one watermark.
ALTER TABLE external_sheet_import_log DROP CONSTRAINT external_sheet_import_log_pkey;
ALTER TABLE external_sheet_import_log ADD PRIMARY KEY (studio_id, spreadsheet_id, tab_name);

-- +goose Down
-- Rolling back keeps only each studio's oldest sheet config and one progress row per sheet+tab.
DELETE FROM studio_external_leads_sheet_settings s
USING (SELECT id, row_number() OVER (PARTITION BY studio_id ORDER BY created_at) AS rn
       FROM studio_external_leads_sheet_settings) d
WHERE s.id = d.id AND d.rn > 1;
ALTER TABLE studio_external_leads_sheet_settings
    DROP CONSTRAINT studio_external_leads_sheet_settings_sheet_key;
ALTER TABLE studio_external_leads_sheet_settings
    ADD CONSTRAINT studio_external_leads_sheet_settings_studio_id_key UNIQUE (studio_id);

DELETE FROM external_sheet_import_log a USING external_sheet_import_log b
WHERE a.ctid < b.ctid AND a.spreadsheet_id = b.spreadsheet_id AND a.tab_name = b.tab_name;
ALTER TABLE external_sheet_import_log DROP CONSTRAINT external_sheet_import_log_pkey;
ALTER TABLE external_sheet_import_log ADD PRIMARY KEY (spreadsheet_id, tab_name);
