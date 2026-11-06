package secaudit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LogEntry is one row as returned to a reader (task 3.6).
type LogEntry struct {
	ID          uuid.UUID      `json:"id"`
	ActorUserID *uuid.UUID     `json:"actorUserId"`
	ActorRole   string         `json:"actorRole"`
	StudioID    *uuid.UUID     `json:"studioId"`
	Action      string         `json:"action"`
	TargetType  string         `json:"targetType"`
	TargetID    string         `json:"targetId"`
	Outcome     string         `json:"outcome"`
	IP          string         `json:"ip"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// ListForStudio returns the most recent entries scoped to one studio —
// the studio_admin read path (task 3.6). super_admin's platform-wide
// read uses ListAll instead; studio_staff has no read path at all
// (enforced by the caller never routing to either of these for that
// role — see the note on ListAll).
func ListForStudio(ctx context.Context, pool *pgxpool.Pool, studioID uuid.UUID, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx, `
		SELECT id, actor_user_id, coalesce(actor_role,''), studio_id, action,
		       coalesce(target_type,''), coalesce(target_id,''), outcome, coalesce(ip,''), created_at
		FROM security_audit_log
		WHERE studio_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, studioID, limit)
	if err != nil {
		return nil, err
	}
	return scanEntries(rows)
}

// ListAll returns the most recent entries platform-wide — the
// super_admin-only read path (task 3.6). Callers MUST gate this behind
// a super_admin role check themselves; this function performs no
// authorization of its own, matching this package's other bare
// query/insert helpers.
func ListAll(ctx context.Context, pool *pgxpool.Pool, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := pool.Query(ctx, `
		SELECT id, actor_user_id, coalesce(actor_role,''), studio_id, action,
		       coalesce(target_type,''), coalesce(target_id,''), outcome, coalesce(ip,''), created_at
		FROM security_audit_log
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return scanEntries(rows)
}

func scanEntries(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}) ([]LogEntry, error) {
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorRole, &e.StudioID, &e.Action,
			&e.TargetType, &e.TargetID, &e.Outcome, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
