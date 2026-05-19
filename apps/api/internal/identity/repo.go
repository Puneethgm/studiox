package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, studio_id, email, password_hash, role, created_at, updated_at
		 FROM users WHERE email = $1`, email)
	return scanUser(row)
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, studio_id, email, password_hash, role, created_at, updated_at
		 FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// UpsertSuperAdmin creates the super-admin user if missing, or updates the
// password hash if it changed. Idempotent — safe to run on every boot.
// Super admins always have NULL studio_id.
func (r *Repo) UpsertSuperAdmin(ctx context.Context, email, passwordHash string) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role)
		VALUES (NULL, $1, $2, 'super_admin')
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    updated_at = now()
		RETURNING id
	`, email, passwordHash)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("upsert super admin: %w", err)
	}
	return id, nil
}

// CreateStudioAdmin inserts a fresh studio_admin user scoped to a studio.
// Returns ErrEmailTaken if the email is already in use.
func (r *Repo) CreateStudioAdmin(ctx context.Context, studioID uuid.UUID, email, passwordHash string) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role)
		VALUES ($1, $2, $3, 'studio_admin')
		RETURNING id
	`, studioID, email, passwordHash)
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

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.StudioID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email already in use")
)
