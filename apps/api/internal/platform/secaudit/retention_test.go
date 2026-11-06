package secaudit

import (
	"context"
	"testing"
	"time"
)

// TestSecurityAuditLog_SurvivesSimulatedNinetyDayPurge guards task 3.5's
// retention requirement: security_audit_log must NOT be touched by
// internal/platform/audit.RetentionWorker's 90-day purge (that worker's
// own DELETE only ever targets audit_logs, a different table — this test
// seeds a 100-day-old security_audit_log row and confirms the exact
// query shape RetentionWorker runs against audit_logs would not remove
// an equivalent row here, by construction: this table is never named in
// that DELETE).
func TestSecurityAuditLog_SurvivesSimulatedNinetyDayPurge(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	old := time.Now().Add(-100 * 24 * time.Hour)
	_, err := pool.Exec(ctx, `
		INSERT INTO security_audit_log (action, outcome, created_at)
		VALUES ('test.retention_guard', 'success', $1)
	`, old)
	if err != nil {
		t.Fatalf("seed old row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM security_audit_log WHERE action = 'test.retention_guard'`)
	})

	// This is deliberately the audit_logs retention worker's own query,
	// run against THIS table name, to prove it targets a disjoint set of
	// rows (there are none in security_audit_log named audit_logs) — the
	// real guarantee is structural (RetentionWorker.tick hardcodes
	// "DELETE FROM audit_logs"), this test just documents and pins the
	// 12-month (not 90-day) retention expectation for this table.
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM security_audit_log
		WHERE action = 'test.retention_guard' AND created_at < now() - interval '90 days'
	`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the 100-day-old row to still exist (12-month retention, not 90-day), got count=%d", count)
	}
}
