package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// CreateStudioUser creates a studio_staff teammate. It never takes a
// password — every teammate starts with DefaultTeammatePassword and
// must_reset_password = true (spec: identity/studio-users). Returns
// ErrEmailTaken or ErrUsernameTaken (the latter only within this studio —
// see the partial unique index on users(studio_id, username)).
func (r *Repo) CreateStudioUser(ctx context.Context, studioID uuid.UUID, username, firstName, lastName, email string, roleID uuid.UUID, createdBy *uuid.UUID) (uuid.UUID, error) {
	passwordHash, err := HashPassword(DefaultTeammatePassword)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash default password: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role, username, first_name,
			last_name, role_id, must_reset_password)
		VALUES ($1, $2, $3, 'studio_staff', $4, $5, $6, $7, true)
		RETURNING id
	`, studioID, email, passwordHash, username, firstName, lastName, roleID)

	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "idx_users_studio_username" {
				return uuid.Nil, ErrUsernameTaken
			}
			return uuid.Nil, ErrEmailTaken
		}
		return uuid.Nil, fmt.Errorf("create studio user: %w", err)
	}
	return id, nil
}

// ListStudioUsers returns every non-deactivated (not soft-deleted) user in
// studioID, along with their role name (NULL for studio_admin, which has
// no role_id). Includes users with active=false — see SetUserActive; that's
// a reversible login gate the Users tab still needs to show/toggle, unlike
// deleted_at which removes them from this list entirely.
func (r *Repo) ListStudioUsers(ctx context.Context, studioID uuid.UUID) ([]StudioUserWithRole, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.email, u.role, u.username, u.first_name, u.last_name,
			u.role_id, sr.name, u.must_reset_password, u.active, u.created_at
		FROM users u
		LEFT JOIN studio_roles sr ON sr.id = u.role_id
		WHERE u.studio_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.created_at ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list studio users: %w", err)
	}
	defer rows.Close()

	var out []StudioUserWithRole
	for rows.Next() {
		var u StudioUserWithRole
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.Username, &u.FirstName, &u.LastName,
			&u.RoleID, &u.RoleName, &u.MustResetPassword, &u.Active, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan studio user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserActive toggles userID's reversible login gate — distinct from
// DeactivateStudioUser's permanent soft-delete. Scoped to studioID so a
// studio_admin can't touch another studio's user by guessing an id. Setting
// active=false doesn't kill that user's already-issued session (same
// staleness trade-off as everything else gated by Claims — see jwt.go); it
// takes effect on their next login attempt.
func (r *Repo) SetUserActive(ctx context.Context, studioID, userID uuid.UUID, active bool, actorID *uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE users
		SET active = $3, updated_by = $4, updated_at = now()
		WHERE id = $1 AND studio_id = $2 AND deleted_at IS NULL
	`, userID, studioID, active, actorID)
	if err != nil {
		return fmt.Errorf("set user active: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeactivateStudioUser soft-deletes a teammate — the row stays, but
// deleted_at/deleted_by are set and FindByEmail will no longer return it
// (so they can't log in). Scoped to studioID so a studio_admin can't
// deactivate another studio's user by guessing an id.
func (r *Repo) DeactivateStudioUser(ctx context.Context, studioID, userID, deactivatedBy uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE users
		SET deleted_at = now(), deleted_by = $3, updated_at = now()
		WHERE id = $1 AND studio_id = $2 AND deleted_at IS NULL
	`, userID, studioID, deactivatedBy)
	if err != nil {
		return fmt.Errorf("deactivate studio user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// StudioUserWithRole is the list-view projection ListStudioUsers returns —
// enough for the Users tab table without a second round-trip per row.
type StudioUserWithRole struct {
	ID                uuid.UUID
	Email             string
	Role              Role
	Username          *string
	FirstName         *string
	LastName          *string
	RoleID            *uuid.UUID
	RoleName          *string
	MustResetPassword bool
	Active            bool
	CreatedAt         time.Time
}
