-- +goose Up
-- Per-studio configuration for polling a read-only, third-party company's
-- Google Sheet for new leads. Column letters are configurable since the
-- sheet's layout is owned by an external party, not us.
CREATE TABLE IF NOT EXISTS studio_external_leads_sheet_settings (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id          uuid        NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    spreadsheet_id     text        NOT NULL DEFAULT '',
    tab_name           text        NOT NULL DEFAULT 'Sheet1',
    name_column        text        NOT NULL DEFAULT '',
    first_name_column  text        NOT NULL DEFAULT 'A',
    last_name_column   text        NOT NULL DEFAULT 'B',
    email_column       text        NOT NULL DEFAULT 'C',
    phone_column       text        NOT NULL DEFAULT 'D',
    source_column      text        NOT NULL DEFAULT '',
    notes_column       text        NOT NULL DEFAULT '',
    active             boolean     NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (studio_id)
);

-- Tracks polling progress (row-number watermark) per configured sheet. We
-- never write back to the external sheet, so progress lives here instead.
CREATE TABLE IF NOT EXISTS external_sheet_import_log (
    spreadsheet_id    text        NOT NULL,
    tab_name          text        NOT NULL,
    last_row_imported int         NOT NULL DEFAULT 1,
    studio_id         uuid        NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (spreadsheet_id, tab_name)
);

-- +goose Down
DROP TABLE IF EXISTS external_sheet_import_log;
DROP TABLE IF EXISTS studio_external_leads_sheet_settings;
