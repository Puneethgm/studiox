package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ListPermissions returns the fixed, platform-wide permission catalog
// (identity/permissions) — the same for every studio.
func (r *Repo) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, key, label FROM permissions ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	var out []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Key, &p.Label); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateRole creates a studio-scoped role. Returns ErrRoleNameTaken if the
// studio already has a role with that name.
func (r *Repo) CreateRole(ctx context.Context, studioID uuid.UUID, name, description string, createdBy *uuid.UUID) (*StudioRole, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO studio_roles (studio_id, name, description, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id, studio_id, name, description, active, created_at, updated_at, created_by, updated_by
	`, studioID, name, description, createdBy)
	return scanStudioRole(row, ErrRoleNameTaken)
}

// UpdateRole renames/redescribes a role. Returns ErrRoleNameTaken if the
// new name collides with another role in the same studio, ErrRoleNotFound
// if roleID doesn't belong to studioID.
func (r *Repo) UpdateRole(ctx context.Context, studioID, roleID uuid.UUID, name, description string, updatedBy *uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE studio_roles
		SET name = $3, description = $4, updated_by = $5, updated_at = now()
		WHERE id = $1 AND studio_id = $2
	`, roleID, studioID, name, description, updatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrRoleNameTaken
		}
		return fmt.Errorf("update role: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// ListRoles returns every role belonging to studioID.
func (r *Repo) ListRoles(ctx context.Context, studioID uuid.UUID) ([]StudioRole, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, name, description, active, created_at, updated_at, created_by, updated_by
		FROM studio_roles
		WHERE studio_id = $1
		ORDER BY name ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var out []StudioRole
	for rows.Next() {
		role, err := scanStudioRole(rows, ErrRoleNameTaken)
		if err != nil {
			return nil, err
		}
		out = append(out, *role)
	}
	return out, rows.Err()
}

// GetRole returns a role, scoped to studioID so one studio can never fetch
// another's role by guessing an id.
func (r *Repo) GetRole(ctx context.Context, studioID, roleID uuid.UUID) (*StudioRole, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, name, description, active, created_at, updated_at, created_by, updated_by
		FROM studio_roles
		WHERE id = $1 AND studio_id = $2
	`, roleID, studioID)
	role, err := scanStudioRole(row, ErrRoleNameTaken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	return role, err
}

// DeleteRole rejects deletion if any user still has this role assigned
// (spec: identity/studio-roles — "a role in use cannot be deleted without
// reassignment"). The FK itself (users.role_id REFERENCES studio_roles(id),
// default ON DELETE RESTRICT) backs this up at the DB layer too.
func (r *Repo) DeleteRole(ctx context.Context, studioID, roleID uuid.UUID) error {
	var inUse int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role_id = $1 AND deleted_at IS NULL`, roleID,
	).Scan(&inUse); err != nil {
		return fmt.Errorf("count users on role: %w", err)
	}
	if inUse > 0 {
		return ErrRoleInUse
	}

	cmd, err := r.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1 AND studio_id = $2`, roleID, studioID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return ErrRoleInUse
		}
		return fmt.Errorf("delete role: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// SetRoleActive toggles roleID's reversible login gate — deactivating it
// blocks login for every user currently assigned to it (checked at login
// time; see IsRoleActive), until reactivated. Scoped to studioID so a
// studio_admin can't touch another studio's role by guessing an id.
func (r *Repo) SetRoleActive(ctx context.Context, studioID, roleID uuid.UUID, active bool, actorID *uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE studio_roles
		SET active = $3, updated_by = $4, updated_at = now()
		WHERE id = $1 AND studio_id = $2
	`, roleID, studioID, active, actorID)
	if err != nil {
		return fmt.Errorf("set role active: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// IsRoleActive is a lightweight check used at login — a studio_staff user
// whose assigned role has been deactivated can't log in even though their
// own user row is still active.
func (r *Repo) IsRoleActive(ctx context.Context, roleID uuid.UUID) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `SELECT active FROM studio_roles WHERE id = $1`, roleID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrRoleNotFound
	}
	if err != nil {
		return false, fmt.Errorf("check role active: %w", err)
	}
	return active, nil
}

// SetRolePermissions replaces roleID's entire permission grant with
// permissionKeys, in one transaction.
func (r *Repo) SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionKeys []string, actorID *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM studio_role_permissions WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("clear role permissions: %w", err)
	}
	if len(permissionKeys) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO studio_role_permissions (role_id, permission_id, created_by)
			SELECT $1, id, $3 FROM permissions WHERE key = ANY($2)
		`, roleID, permissionKeys, actorID); err != nil {
			return fmt.Errorf("insert role permissions: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// GetRolePermissionKeys returns the permission keys granted to roleID —
// used to populate Claims.Permissions at login.
func (r *Repo) GetRolePermissionKeys(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.key
		FROM studio_role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
	`, roleID)
	if err != nil {
		return nil, fmt.Errorf("get role permission keys: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan permission key: %w", err)
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func scanStudioRole(row pgx.Row, onUniqueViolation error) (*StudioRole, error) {
	var role StudioRole
	if err := row.Scan(&role.ID, &role.StudioID, &role.Name, &role.Description, &role.Active,
		&role.CreatedAt, &role.UpdatedAt, &role.CreatedBy, &role.UpdatedBy); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, onUniqueViolation
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("scan studio role: %w", err)
	}
	return &role, nil
}
