-- +goose Up
-- +goose StatementBegin
ALTER TABLE tree_nodes ADD COLUMN position_x DOUBLE PRECISION;
ALTER TABLE tree_nodes ADD COLUMN position_y DOUBLE PRECISION;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tree_nodes DROP COLUMN position_x;
ALTER TABLE tree_nodes DROP COLUMN position_y;
-- +goose StatementEnd
