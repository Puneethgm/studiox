package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const userColumns = `id, studio_id, email, password_hash, role, username, first_name,
	last_name, role_id, must_reset_password, active, deleted_at, deleted_by, created_at, updated_at`

// FindByEmail excludes deactivated (soft-deleted) users — a deactivated
// teammate can no longer log in (spec: identity/studio-users).
func (r *Repo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
	return scanUser(row)
}

// FindByID excludes deactivated users, matching FindByEmail — a deactivated
// user's still-valid JWT must not keep working for /me, password changes,
// or permission checks (RequirePermission) just because it hasn't expired.
func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanUser(row)
}

// UpdatePasswordHash also clears must_reset_password — any successful
// password change, forced or voluntary, satisfies the reset requirement
// (design.md decision 4's correction: reuses the existing change-password
// flow instead of a separate set-password endpoint). actorID is always the
// user themselves — there is no "admin resets a teammate's password"
// feature — but it's still threaded explicitly rather than assumed, per
// design.md decision 8.
func (r *Repo) UpdatePasswordHash(ctx context.Context, id uuid.UUID, passwordHash string, actorID *uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    must_reset_password = false,
		    updated_by = $3,
		    updated_at = now()
		WHERE id = $1
	`, id, passwordHash, actorID)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreatePasswordResetToken issues a new single-use reset token for userID,
// valid for ttl. The raw token is returned only here (to embed in the email
// link) — the DB only ever stores its SHA-256 hash, so a database leak
// alone can't be used to reset anyone's password.
func (r *Repo) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	_, err := r.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + $3)
	`, userID, hash, ttl)
	if err != nil {
		return "", fmt.Errorf("insert reset token: %w", err)
	}
	return token, nil
}

// ConsumePasswordResetToken validates a raw token (unexpired, unused) and
// atomically marks it used while updating the user's password — a token can
// never be replayed even if the two statements below race with a second
// request for the same token, since the UPDATE's WHERE clause only matches
// while used_at IS NULL.
func (r *Repo) ConsumePasswordResetToken(ctx context.Context, rawToken, newPasswordHash string) error {
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE password_reset_tokens
		SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, hash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrResetTokenInvalid
		}
		return fmt.Errorf("consume reset token: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, must_reset_password = false, updated_at = now()
		WHERE id = $1
	`, userID, newPasswordHash); err != nil {
		return fmt.Errorf("update password from reset token: %w", err)
	}

	return tx.Commit(ctx)
}

// GetStudioName looks up just a studio's display name — identity can't
// import the studios package (studios already imports identity for RBAC
// checks, so that direction would cycle), so this is a narrow direct query
// rather than a cross-package call, used only to personalize transactional
// email copy.
func (r *Repo) GetStudioName(ctx context.Context, studioID uuid.UUID) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, `SELECT name FROM studios WHERE id = $1`, studioID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get studio name: %w", err)
	}
	return name, nil
}

// UpsertSuperAdmin creates the super-admin user if missing, or updates the
// password hash if it changed. Idempotent — safe to run on every boot, from
// cmd/seed with no logged-in actor — actorID is always nil in practice.
// Super admins always have NULL studio_id.
func (r *Repo) UpsertSuperAdmin(ctx context.Context, email, passwordHash string, actorID *uuid.UUID) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role, created_by, updated_by)
		VALUES (NULL, $1, $2, 'super_admin', $3, $3)
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    updated_by = $3,
		    updated_at = now()
		RETURNING id
	`, email, passwordHash, actorID)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("upsert super admin: %w", err)
	}
	return id, nil
}

// CreateStudioAdmin inserts a fresh studio_admin user scoped to a studio.
// Returns ErrEmailTaken if the email is already in use. Currently unused
// (studio+first-admin creation goes through studios.Repo.Create directly)
// but kept in sync with the actorID convention since it's a real write path.
func (r *Repo) CreateStudioAdmin(ctx context.Context, studioID uuid.UUID, email, passwordHash string, actorID *uuid.UUID) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role, created_by, updated_by)
		VALUES ($1, $2, $3, 'studio_admin', $4, $4)
		RETURNING id
	`, studioID, email, passwordHash, actorID)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, ErrEmailTaken
		}
		return uuid.Nil, fmt.Errorf("create studio admin: %w", err)
	}
	return id, nil
}

// ListByStudioID excludes deactivated users — see ListStudioUsers for the
// same listing including their assigned role name.
func (r *Repo) ListByStudioID(ctx context.Context, studioID uuid.UUID) ([]User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE studio_id = $1 AND deleted_at IS NULL
		ORDER BY email ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list users by studio: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.StudioID, &u.Email, &u.PasswordHash, &u.Role, &u.Username,
		&u.FirstName, &u.LastName, &u.RoleID, &u.MustResetPassword, &u.Active, &u.DeletedAt, &u.DeletedBy,
		&u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

var (
	ErrNotFound      = errors.New("user not found")
	ErrEmailTaken    = errors.New("email already in use")
	ErrUsernameTaken = errors.New("username already taken in this studio")
	ErrRoleNameTaken = errors.New("role name already exists in this studio")
	ErrRoleInUse     = errors.New("role is still assigned to one or more users")
	ErrRoleNotFound  = errors.New("role not found")

	ErrResetTokenInvalid = errors.New("reset token is invalid or expired")
)
