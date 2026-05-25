-- +goose Up
-- +goose StatementBegin
ALTER TABLE leads ADD COLUMN assigned_to TEXT;
ALTER TABLE leads ADD COLUMN trial_attended BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE leads ADD COLUMN member_sold BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE leads ADD COLUMN monthly_fee NUMERIC(10,2) NOT NULL DEFAULT 0.00;
ALTER TABLE leads ADD COLUMN offer TEXT;
ALTER TABLE leads ADD COLUMN further_notes TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE leads DROP COLUMN IF EXISTS further_notes;
ALTER TABLE leads DROP COLUMN IF EXISTS offer;
ALTER TABLE leads DROP COLUMN IF EXISTS monthly_fee;
ALTER TABLE leads DROP COLUMN IF EXISTS member_sold;
ALTER TABLE leads DROP COLUMN IF EXISTS trial_attended;
ALTER TABLE leads DROP COLUMN IF EXISTS assigned_to;
-- +goose StatementEnd
