// Package dbrls sets the Postgres session variables platform-security-
// hardening task 7.1's RLS policies (leads, messages — see
// migrations/20261030130001_rls_first_tables.sql) read, and task 9.1's
// extension of them to the rest of the studio-scoped tables.
//
// This codebase has no per-request transaction wrapping today — most
// repo methods run bare pool.Query/pool.Exec calls, not inside an
// explicit transaction scoped to one request — so this package uses
// `SET LOCAL` inside a transaction it opens itself, rather than a bare
// `SET` on a pooled connection (which would leak the setting to
// whichever unrelated request reuses that connection next, since
// pgxpool does not reset custom GUCs between acquisitions). `SET
// LOCAL`'s effect is automatically undone at COMMIT/ROLLBACK, so there
// is no leakage risk regardless of pooling.
package dbrls

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BeginScopedToStudio opens a transaction with app.current_studio_id set
// for its lifetime, so every RLS-enabled table's policy restricts rows
// to that studio for every query run on the returned tx. The caller
// must Commit or Rollback it.
func BeginScopedToStudio(ctx context.Context, pool *pgxpool.Pool, studioID uuid.UUID) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_studio_id = '%s'", studioID.String())); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set app.current_studio_id: %w", err)
	}
	return tx, nil
}

// BeginBypassingRLS opens a transaction with app.bypass_rls set for its
// lifetime — the super_admin escape hatch task 7.1 requires. Callers
// MUST have already confirmed the caller is authenticated as
// super_admin before calling this; this function performs no
// authorization check of its own, matching this codebase's other bare
// query helpers.
func BeginBypassingRLS(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL app.bypass_rls = 'true'"); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set app.bypass_rls: %w", err)
	}
	return tx, nil
}
