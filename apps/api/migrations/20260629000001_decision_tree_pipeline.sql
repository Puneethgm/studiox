-- +goose Up
-- +goose StatementBegin

-- Add action_value column to store pipeline action configuration (e.g. target status for change_status)
ALTER TABLE tree_nodes ADD COLUMN action_value JSONB NOT NULL DEFAULT '{}';

-- Widen condition_type to allow lead_status filtering
ALTER TABLE tree_nodes DROP CONSTRAINT tree_nodes_condition_type_check;
ALTER TABLE tree_nodes ADD CONSTRAINT tree_nodes_condition_type_check
  CHECK (condition_type IN ('keyword','intent','sentiment','default','lead_status'));

-- Widen action to allow change_status pipeline action
ALTER TABLE tree_nodes DROP CONSTRAINT tree_nodes_action_check;
ALTER TABLE tree_nodes ADD CONSTRAINT tree_nodes_action_check
  CHECK (action IN ('reply','escalate_human','book_trial','send_link','change_status'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tree_nodes DROP CONSTRAINT tree_nodes_condition_type_check;
ALTER TABLE tree_nodes ADD CONSTRAINT tree_nodes_condition_type_check
  CHECK (condition_type IN ('keyword','intent','sentiment','default'));

ALTER TABLE tree_nodes DROP CONSTRAINT tree_nodes_action_check;
ALTER TABLE tree_nodes ADD CONSTRAINT tree_nodes_action_check
  CHECK (action IN ('reply','escalate_human','book_trial','send_link'));

ALTER TABLE tree_nodes DROP COLUMN action_value;

-- +goose StatementEnd
