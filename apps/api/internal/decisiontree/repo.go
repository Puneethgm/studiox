package decisiontree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ----- trees -----

func (r *Repo) CreateTree(ctx context.Context, studioID uuid.UUID, input CreateTreeInput) (*Tree, error) {
	t := &Tree{}
	statuses := input.TargetStatuses
	if statuses == nil {
		statuses = []string{}
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO decision_trees (studio_id, name, target_statuses)
		VALUES ($1, $2, $3)
		RETURNING id, studio_id, name, is_active, target_statuses, created_at, updated_at
	`, studioID, input.Name, statuses).Scan(
		&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}
	return t, nil
}

func (r *Repo) ListTrees(ctx context.Context, studioID uuid.UUID) ([]Tree, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, name, is_active, target_statuses, created_at, updated_at
		FROM decision_trees
		WHERE studio_id = $1
		ORDER BY created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list trees: %w", err)
	}
	defer rows.Close()

	trees := []Tree{}
	for rows.Next() {
		var t Tree
		if err := rows.Scan(&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tree: %w", err)
		}
		trees = append(trees, t)
	}
	return trees, nil
}

func (r *Repo) GetTree(ctx context.Context, studioID, treeID uuid.UUID) (*Tree, error) {
	t := &Tree{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, name, is_active, target_statuses, created_at, updated_at
		FROM decision_trees
		WHERE id = $1 AND studio_id = $2
	`, treeID, studioID).Scan(
		&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTreeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	nodes, err := r.listNodes(ctx, treeID)
	if err != nil {
		return nil, err
	}
	t.Nodes = buildTree(nodes)
	return t, nil
}

// GetActiveTreeForLead finds the best active tree for a lead's pipeline status.
// Specific-status trees (target_statuses non-empty matching lead) take priority over
// catch-all trees (target_statuses empty). Returns ErrTreeNotFound if none match.
func (r *Repo) GetActiveTreeForLead(ctx context.Context, studioID uuid.UUID, leadStatus string) (*Tree, error) {
	t := &Tree{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, name, is_active, target_statuses, created_at, updated_at
		FROM decision_trees
		WHERE studio_id = $1
		  AND is_active = TRUE
		  AND (target_statuses = '{}' OR $2 = ANY(target_statuses))
		ORDER BY
			CASE WHEN target_statuses != '{}' THEN 0 ELSE 1 END ASC,
			created_at DESC
		LIMIT 1
	`, studioID, leadStatus).Scan(
		&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTreeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active tree: %w", err)
	}
	nodes, err := r.listNodes(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t.Nodes = buildTree(nodes)
	return t, nil
}

func (r *Repo) UpdateTree(ctx context.Context, studioID, treeID uuid.UUID, input UpdateTreeInput) (*Tree, error) {
	t := &Tree{}
	var err error
	if input.UpdateStatuses {
		statuses := input.TargetStatuses
		if statuses == nil {
			statuses = []string{}
		}
		err = r.pool.QueryRow(ctx, `
			UPDATE decision_trees
			SET
				name            = COALESCE($3, name),
				is_active       = COALESCE($4, is_active),
				target_statuses = $5,
				updated_at      = now()
			WHERE id = $1 AND studio_id = $2
			RETURNING id, studio_id, name, is_active, target_statuses, created_at, updated_at
		`, treeID, studioID, input.Name, input.IsActive, statuses).Scan(
			&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt,
		)
	} else {
		err = r.pool.QueryRow(ctx, `
			UPDATE decision_trees
			SET
				name       = COALESCE($3, name),
				is_active  = COALESCE($4, is_active),
				updated_at = now()
			WHERE id = $1 AND studio_id = $2
			RETURNING id, studio_id, name, is_active, target_statuses, created_at, updated_at
		`, treeID, studioID, input.Name, input.IsActive).Scan(
			&t.ID, &t.StudioID, &t.Name, &t.IsActive, &t.TargetStatuses, &t.CreatedAt, &t.UpdatedAt,
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTreeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update tree: %w", err)
	}
	return t, nil
}

func (r *Repo) DeleteTree(ctx context.Context, studioID, treeID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM decision_trees WHERE id = $1 AND studio_id = $2
	`, treeID, studioID)
	if err != nil {
		return fmt.Errorf("delete tree: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTreeNotFound
	}
	return nil
}

// ----- nodes -----

func (r *Repo) listNodes(ctx context.Context, treeID uuid.UUID) ([]Node, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tree_id, parent_id, label, condition_type, condition_value,
		       reply_template, action, action_value, sort_order, created_at, updated_at
		FROM tree_nodes
		WHERE tree_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, treeID)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	nodes := []Node{}
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (r *Repo) CreateNode(ctx context.Context, input CreateNodeInput) (*Node, error) {
	cvJSON, err := json.Marshal(input.ConditionValue)
	if err != nil {
		return nil, fmt.Errorf("marshal condition value: %w", err)
	}
	avJSON, err := json.Marshal(input.ActionValue)
	if err != nil {
		return nil, fmt.Errorf("marshal action value: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO tree_nodes
		    (tree_id, parent_id, label, condition_type, condition_value, reply_template, action, action_value, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, tree_id, parent_id, label, condition_type, condition_value,
		          reply_template, action, action_value, sort_order, created_at, updated_at
	`, input.TreeID, input.ParentID, input.Label, string(input.ConditionType),
		string(cvJSON), input.ReplyTemplate, string(input.Action), string(avJSON), input.SortOrder)
	n, err := scanNode(row)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return &n, nil
}

func (r *Repo) UpdateNode(ctx context.Context, treeID, nodeID uuid.UUID, input UpdateNodeInput) (*Node, error) {
	var cvJSON *string
	if input.ConditionValue != nil {
		b, err := json.Marshal(input.ConditionValue)
		if err != nil {
			return nil, fmt.Errorf("marshal condition value: %w", err)
		}
		s := string(b)
		cvJSON = &s
	}
	var avJSON *string
	if input.ActionValue != nil {
		b, err := json.Marshal(input.ActionValue)
		if err != nil {
			return nil, fmt.Errorf("marshal action value: %w", err)
		}
		s := string(b)
		avJSON = &s
	}

	var ct *string
	if input.ConditionType != nil {
		s := string(*input.ConditionType)
		ct = &s
	}
	var act *string
	if input.Action != nil {
		s := string(*input.Action)
		act = &s
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tree_nodes SET
			label           = COALESCE($3, label),
			condition_type  = COALESCE($4, condition_type),
			condition_value = COALESCE($5::jsonb, condition_value),
			reply_template  = COALESCE($6, reply_template),
			action          = COALESCE($7, action),
			action_value    = COALESCE($8::jsonb, action_value),
			sort_order      = COALESCE($9, sort_order),
			updated_at      = now()
		WHERE id = $1 AND tree_id = $2
		RETURNING id, tree_id, parent_id, label, condition_type, condition_value,
		          reply_template, action, action_value, sort_order, created_at, updated_at
	`, nodeID, treeID, input.Label, ct, cvJSON, input.ReplyTemplate, act, avJSON, input.SortOrder)

	n, err := scanNode(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update node: %w", err)
	}
	return &n, nil
}

func (r *Repo) DeleteNode(ctx context.Context, treeID, nodeID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM tree_nodes WHERE id = $1 AND tree_id = $2
	`, nodeID, treeID)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// ----- helpers -----

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(s scanner) (Node, error) {
	var n Node
	var cvRaw, avRaw []byte
	var ct, act string
	err := s.Scan(
		&n.ID, &n.TreeID, &n.ParentID, &n.Label,
		&ct, &cvRaw,
		&n.ReplyTemplate, &act, &avRaw, &n.SortOrder,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return n, err
	}
	n.ConditionType = ConditionType(ct)
	n.Action = Action(act)
	if len(cvRaw) > 0 {
		_ = json.Unmarshal(cvRaw, &n.ConditionValue)
	}
	if len(avRaw) > 0 {
		_ = json.Unmarshal(avRaw, &n.ActionValue)
	}
	return n, nil
}

// buildTree converts a flat list of nodes into a nested tree by parentID.
// Uses a recursive approach so children at all depths are correctly populated.
func buildTree(flat []Node) []Node {
	// Build a map from parentID → list of child indices
	childrenOf := make(map[uuid.UUID][]int, len(flat))
	roots := []int{}
	for i := range flat {
		if flat[i].ParentID == nil {
			roots = append(roots, i)
		} else {
			pid := *flat[i].ParentID
			childrenOf[pid] = append(childrenOf[pid], i)
		}
	}

	var build func(idx int) Node
	build = func(idx int) Node {
		n := flat[idx]
		n.Children = []Node{}
		for _, ci := range childrenOf[n.ID] {
			n.Children = append(n.Children, build(ci))
		}
		return n
	}

	result := make([]Node, 0, len(roots))
	for _, i := range roots {
		result = append(result, build(i))
	}
	return result
}
