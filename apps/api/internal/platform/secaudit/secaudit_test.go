package secaudit

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestLog_RoundTripsAllFields(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	logger := New(pool)

	actorID := uuid.New()
	studioID := uuid.New()
	sessionID := uuid.New()

	err := logger.Log(ctx, Entry{
		ActorUserID: &actorID,
		ActorRole:   "super_admin",
		StudioID:    &studioID,
		Action:      "test.round_trip",
		TargetType:  "user",
		TargetID:    "some-target",
		Metadata:    map[string]any{"foo": "bar"},
		IP:          "127.0.0.1",
		UserAgent:   "test-agent",
		Country:     "SG",
		SessionID:   &sessionID,
		Outcome:     OutcomeSuccess,
	})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM security_audit_log WHERE action = 'test.round_trip'`)
	})

	var gotRole, gotTargetType, gotTargetID, gotIP, gotUA, gotCountry, gotOutcome string
	var gotActor, gotStudio, gotSession uuid.UUID
	var gotMeta []byte
	err = pool.QueryRow(ctx, `
		SELECT actor_user_id, actor_role, studio_id, target_type, target_id, ip, user_agent, country, session_id, outcome, metadata
		FROM security_audit_log WHERE action = 'test.round_trip'
	`).Scan(&gotActor, &gotRole, &gotStudio, &gotTargetType, &gotTargetID, &gotIP, &gotUA, &gotCountry, &gotSession, &gotOutcome, &gotMeta)
	if err != nil {
		t.Fatalf("select back: %v", err)
	}
	if gotActor != actorID || gotRole != "super_admin" || gotStudio != studioID {
		t.Errorf("actor/role/studio mismatch: %v %v %v", gotActor, gotRole, gotStudio)
	}
	if gotTargetType != "user" || gotTargetID != "some-target" {
		t.Errorf("target mismatch: %v %v", gotTargetType, gotTargetID)
	}
	if gotIP != "127.0.0.1" || gotUA != "test-agent" || gotCountry != "SG" {
		t.Errorf("ip/ua/country mismatch: %v %v %v", gotIP, gotUA, gotCountry)
	}
	if gotSession != sessionID || gotOutcome != OutcomeSuccess {
		t.Errorf("session/outcome mismatch: %v %v", gotSession, gotOutcome)
	}
	if string(gotMeta) != `{"foo": "bar"}` && string(gotMeta) != `{"foo":"bar"}` {
		t.Errorf("metadata mismatch: %s", gotMeta)
	}
}

func TestLog_MissingActionOrOutcomeRejected(t *testing.T) {
	pool := testPool(t)
	logger := New(pool)
	ctx := context.Background()

	if err := logger.Log(ctx, Entry{Outcome: OutcomeSuccess}); err == nil {
		t.Error("expected error for missing Action")
	}
	if err := logger.Log(ctx, Entry{Action: "x"}); err == nil {
		t.Error("expected error for missing Outcome")
	}
}

func TestLogTx_RollsBackWithTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := LogTx(ctx, tx, Entry{Action: "test.tx_rollback", Outcome: OutcomeSuccess}); err != nil {
		t.Fatalf("LogTx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM security_audit_log WHERE action = 'test.tx_rollback'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected rolled-back entry to not exist, got count=%d", count)
	}
}

func TestLogTx_CommitsWithTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := LogTx(ctx, tx, Entry{Action: "test.tx_commit", Outcome: OutcomeSuccess}); err != nil {
		t.Fatalf("LogTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM security_audit_log WHERE action = 'test.tx_commit'`)
	})

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM security_audit_log WHERE action = 'test.tx_commit'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected committed entry to exist, got count=%d", count)
	}
}
