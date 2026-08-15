-- +goose Up
ALTER TABLE conversations ADD COLUMN current_tree_node_id UUID REFERENCES tree_nodes(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE conversations DROP COLUMN current_tree_node_id;
