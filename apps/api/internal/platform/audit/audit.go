// Package audit provides a minimal, best-effort audit trail used by
// compliance-sensitive flows (currently: studio account deletion, archive
// creation, recovery attempts — see init.md §4.5). Writes are simple
// inserts with no retry/queue, matching this codebase's existing treatment
// of non-critical side effects (see internal/platform/mail's package doc
// comment): a failed audit write must never abort the primary operation it
// describes, just get logged.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

// Log records one audit-trail entry. It returns the error so a caller that
// cares can inspect it, but every call site in the account-deletion/
// recovery flow treats this as fire-and-forget-but-logged — log the error
// via slog and keep going, never abort the primary operation because the
// audit write itself failed (see this package's doc comment).
func (l *Logger) Log(ctx context.Context, studioID *uuid.UUID, actorUserID *uuid.UUID, action string, metadata map[string]any) error {
	if l == nil || l.pool == nil {
		return fmt.Errorf("audit logger not configured")
	}

	metaJSON := []byte("{}")
	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			slog.Error("audit log: failed to marshal metadata", "action", action, "err", err)
		} else {
			metaJSON = encoded
		}
	}

	_, err := l.pool.Exec(ctx, `
		INSERT INTO audit_logs (studio_id, actor_user_id, action, metadata)
		VALUES ($1, $2, $3, $4)
	`, studioID, actorUserID, action, metaJSON)
	if err != nil {
		slog.Error("audit log: insert failed", "action", action, "err", err)
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
