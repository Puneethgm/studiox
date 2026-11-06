// Package secaudit is platform-security-hardening's security event log —
// deliberately a separate package/table from internal/platform/audit
// (audit_logs), which is owned by the unrelated add-studio-rbac-and-
// audit-trail change and records row-provenance for compliance flows, not
// security events. This log is append-only: no handler in this codebase
// exposes an edit/delete path for security_audit_log, and it is retained
// for 12 months rather than audit_logs' 90 days (see
// internal/platform/audit/retention_worker.go, which only ever targets
// audit_logs and must never be pointed at this table).
package secaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Outcome values for the outcome column. Kept as a small open string
// rather than a Postgres enum so new outcomes don't need a migration.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
	OutcomeDenied  = "denied"
)

// Entry is one security-audit-log row to write. Every field besides
// Action and Outcome is optional (nil-able pointers / empty string),
// since not every call site has a studio, a target, or a session.
type Entry struct {
	ActorUserID *uuid.UUID
	ActorRole   string
	StudioID    *uuid.UUID
	Action      string
	TargetType  string
	TargetID    string
	Metadata    map[string]any
	IP          string
	UserAgent   string
	Country     string
	SessionID   *uuid.UUID
	Outcome     string
}

// Logger writes Entry rows to security_audit_log. A nil *Logger is
// treated as "not configured" by Log, matching this codebase's existing
// internal/platform/audit.Logger nil-safety convention, so call sites
// that don't have a logger wired yet don't need an extra nil check.
type Logger struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

// Log inserts one entry using the Logger's own pool. For call sites that
// need the write to participate in an existing transaction (task 3.4's
// same-transaction subset), use LogTx instead with that transaction's
// pgx.Tx.
func (l *Logger) Log(ctx context.Context, e Entry) error {
	if l == nil || l.pool == nil {
		return fmt.Errorf("secaudit logger not configured")
	}
	return insert(ctx, l.pool, e)
}

// LogTx inserts one entry using the given transaction, so a failed insert
// rolls back whatever else that transaction did — the mechanism behind
// task 3.4. The caller owns Commit/Rollback as usual; LogTx only executes
// the INSERT.
func LogTx(ctx context.Context, tx pgx.Tx, e Entry) error {
	return insert(ctx, tx, e)
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, so insert is
// shared between Log and LogTx instead of duplicated.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insert(ctx context.Context, q querier, e Entry) error {
	if e.Action == "" {
		return fmt.Errorf("secaudit: Action is required")
	}
	if e.Outcome == "" {
		return fmt.Errorf("secaudit: Outcome is required")
	}

	metaJSON := []byte("{}")
	if e.Metadata != nil {
		encoded, err := json.Marshal(e.Metadata)
		if err != nil {
			slog.Error("secaudit: failed to marshal metadata", "action", e.Action, "err", err)
		} else {
			metaJSON = encoded
		}
	}

	_, err := q.Exec(ctx, `
		INSERT INTO security_audit_log (
			actor_user_id, actor_role, studio_id, action, target_type, target_id,
			metadata, ip, user_agent, country, session_id, outcome
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		e.ActorUserID, nullStr(e.ActorRole), e.StudioID, e.Action, nullStr(e.TargetType), nullStr(e.TargetID),
		metaJSON, nullStr(e.IP), nullStr(e.UserAgent), nullStr(e.Country), e.SessionID, e.Outcome,
	)
	if err != nil {
		slog.Error("secaudit: insert failed", "action", e.Action, "err", err)
		return fmt.Errorf("insert security audit log: %w", err)
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
