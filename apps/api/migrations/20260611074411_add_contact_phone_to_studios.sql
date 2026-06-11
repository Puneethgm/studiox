-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN contact_phone TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS contact_phone;
-- +goose StatementEnd
