-- +goose Up
-- +goose StatementBegin

-- target_statuses: which pipeline stages this tree responds to.
-- Empty array = responds to ALL leads (catch-all).
-- Non-empty = only responds to leads whose status is in the array.
ALTER TABLE decision_trees ADD COLUMN target_statuses TEXT[] NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE decision_trees DROP COLUMN target_statuses;
-- +goose StatementEnd
