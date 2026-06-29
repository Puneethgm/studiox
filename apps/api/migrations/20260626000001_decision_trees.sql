-- +goose Up
-- +goose StatementBegin
CREATE TABLE decision_trees (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id  UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_decision_trees_studio ON decision_trees(studio_id);

CREATE TABLE tree_nodes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tree_id          UUID NOT NULL REFERENCES decision_trees(id) ON DELETE CASCADE,
    parent_id        UUID REFERENCES tree_nodes(id) ON DELETE CASCADE,
    label            TEXT NOT NULL,
    condition_type   TEXT NOT NULL CHECK (condition_type IN ('keyword','intent','sentiment','default')),
    condition_value  JSONB NOT NULL DEFAULT '{}',
    reply_template   TEXT NOT NULL DEFAULT '',
    action           TEXT NOT NULL DEFAULT 'reply' CHECK (action IN ('reply','escalate_human','book_trial','send_link')),
    sort_order       INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tree_nodes_tree ON tree_nodes(tree_id);
CREATE INDEX idx_tree_nodes_parent ON tree_nodes(parent_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tree_nodes;
DROP TABLE IF EXISTS decision_trees;
-- +goose StatementEnd
