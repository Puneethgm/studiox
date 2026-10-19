package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	retentionTickInterval = 24 * time.Hour
	// retentionWindow is the minimum retention period from init.md §4.5
	// ("all system and account-related logs retained ≥90 days"). Rows older
	// than this are purged; payment/financial records live in a separate
	// table (retained_payment_records) with its own, much longer retention
	// and are never touched by this worker.
	retentionWindow = 90 * 24 * time.Hour
)

// RetentionWorker enforces the 90-day minimum retention window on
// audit_logs by deleting anything older, once per day. Mirrors the
// ticker-based Run(ctx)/tick(ctx) shape of
// internal/messaging/broadcast_worker.go.
type RetentionWorker struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewRetentionWorker(pool *pgxpool.Pool, log *slog.Logger) *RetentionWorker {
	return &RetentionWorker{pool: pool, log: log}
}

func (w *RetentionWorker) Run(ctx context.Context) {
	w.log.Info("Audit retention worker started", "component", "audit_retention", "tick_interval", retentionTickInterval, "retention_window", retentionWindow)

	w.tick(ctx)

	t := time.NewTicker(retentionTickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("Audit retention worker stopping", "component", "audit_retention")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *RetentionWorker) tick(ctx context.Context) {
	tag, err := w.pool.Exec(ctx, `DELETE FROM audit_logs WHERE created_at < now() - interval '90 days'`)
	if err != nil {
		w.log.Warn("Audit retention — delete failed, will retry next tick",
			"component", "audit_retention", "error", err.Error())
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		w.log.Info("Audit retention — purged expired rows", "component", "audit_retention", "rows_deleted", n)
	}
}
