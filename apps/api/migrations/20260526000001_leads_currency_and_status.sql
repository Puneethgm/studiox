-- +goose Up
-- +goose StatementBegin
ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_status_check;
ALTER TABLE leads ADD CONSTRAINT leads_status_check CHECK (status IN ('new','contacted','trial_booked','member','dropped','paused'));
ALTER TABLE leads ADD COLUMN currency TEXT NOT NULL DEFAULT 'SGD';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE leads DROP COLUMN IF EXISTS currency;
ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_status_check;
ALTER TABLE leads ADD CONSTRAINT leads_status_check CHECK (status IN ('new','contacted','trial_booked','member','dropped'));
-- +goose StatementEnd
