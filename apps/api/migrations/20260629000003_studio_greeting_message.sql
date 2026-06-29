-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios ADD COLUMN greeting_message TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN greeting_message;
-- +goose StatementEnd
