package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

func TestSGTDayBounds(t *testing.T) {
	// 2026-01-15 23:30 UTC is already 2026-01-16 07:30 in Singapore (UTC+8),
	// so it must bucket into the 16th's [start, end) window, not the 15th's.
	utc := time.Date(2026, 1, 15, 23, 30, 0, 0, time.UTC)
	start, end := sgtDayBounds(utc)

	wantStart := time.Date(2026, 1, 16, 0, 0, 0, 0, sgtZone)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(start.Add(24 * time.Hour)) {
		t.Errorf("end = %v, want start+24h = %v", end, start.Add(24*time.Hour))
	}
	if !utc.Before(end) || !utc.After(start.Add(-time.Nanosecond)) {
		t.Errorf("input instant %v not within [%v, %v)", utc, start, end)
	}

	// A second call one hour later, still the same SGT calendar day, must
	// yield the same bucket.
	start2, end2 := sgtDayBounds(utc.Add(1 * time.Hour))
	if !start2.Equal(start) || !end2.Equal(end) {
		t.Errorf("bucket drifted within the same SGT day: got [%v,%v), want [%v,%v)", start2, end2, start, end)
	}
}

// setupDailyLimitTestEnv connects to the real dev DB (same .env-driven
// pattern as the other integration tests in this package) and finds an
// existing studio with an active WhatsApp channel + a conversation to attach
// outbound_jobs fixtures to.
func setupDailyLimitTestEnv(t *testing.T) (*Repo, *pgxpool.Pool, uuid.UUID, uuid.UUID) {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	var studioID, conversationID uuid.UUID
	err = pool.QueryRow(ctx, `
		SELECT c.studio_id, c.id
		FROM conversations c
		JOIN channel_accounts ca ON ca.id = c.channel_account_id
		WHERE ca.kind IN ('whatsapp_meta', 'whatsapp_web')
		LIMIT 1
	`).Scan(&studioID, &conversationID)
	if err != nil {
		t.Skip("Skipping test; no conversation on a WhatsApp channel found in DB")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)
	return repo, pool, studioID, conversationID
}

func TestWhatsAppDailyMessageLimit_GetSetRoundtrip(t *testing.T) {
	repo, _, studioID, _ := setupDailyLimitTestEnv(t)
	ctx := context.Background()

	original, err := repo.GetWhatsAppDailyMessageLimit(ctx, studioID)
	if err != nil {
		t.Fatalf("GetWhatsAppDailyMessageLimit: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.SetWhatsAppDailyMessageLimit(ctx, studioID, original); err != nil {
			t.Errorf("restore original limit: %v", err)
		}
	})

	if err := repo.SetWhatsAppDailyMessageLimit(ctx, studioID, 5); err != nil {
		t.Fatalf("SetWhatsAppDailyMessageLimit: %v", err)
	}
	got, err := repo.GetWhatsAppDailyMessageLimit(ctx, studioID)
	if err != nil {
		t.Fatalf("GetWhatsAppDailyMessageLimit after set: %v", err)
	}
	if got != 5 {
		t.Errorf("limit after set = %d, want 5", got)
	}
}

// TestCountAutomatedWhatsAppSentToday_OnlyCountsAutomatedSentTodayOnWhatsApp
// inserts fixture messages/outbound_jobs covering every case the count query
// must discriminate: automated vs manual source, sent vs pending status, and
// today vs a day outside the SGT window — and asserts only the one row that
// matches all three conditions is counted.
func TestCountAutomatedWhatsAppSentToday_OnlyCountsAutomatedSentTodayOnWhatsApp(t *testing.T) {
	repo, pool, studioID, conversationID := setupDailyLimitTestEnv(t)
	ctx := context.Background()

	before, err := repo.CountAutomatedWhatsAppSentToday(ctx, studioID)
	if err != nil {
		t.Fatalf("CountAutomatedWhatsAppSentToday (baseline): %v", err)
	}

	insertJob := func(sourceKind SourceKind, status string, sentAt any) uuid.UUID {
		id := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments, source_kind, status, sent_at, source_ref)
			VALUES ($1, $2, 'test body', '[]', $3, $4, $5, $6)
		`, studioID, conversationID, sourceKind, status, sentAt, "daily_limit_test:"+id.String())
		if err != nil {
			t.Fatalf("insert fixture outbound_job: %v", err)
		}
		return id
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM outbound_jobs WHERE source_ref LIKE 'daily_limit_test:%'`); err != nil {
			t.Errorf("cleanup fixture outbound_jobs: %v", err)
		}
	})

	now := time.Now().UTC()
	yesterdaySGT, _ := sgtDayBounds(now.Add(-24 * time.Hour))

	insertJob(SourceAutomation, "sent", now)                         // counts
	insertJob(SourceAI, "sent", now)                                 // counts
	insertJob(SourceStudioUser, "sent", now)                         // manual — must not count
	insertJob(SourceAutomation, "pending", nil)                      // not sent yet — must not count
	insertJob(SourceAutomation, "sent", yesterdaySGT.Add(time.Hour)) // sent, but outside today's SGT window — must not count

	got, err := repo.CountAutomatedWhatsAppSentToday(ctx, studioID)
	if err != nil {
		t.Fatalf("CountAutomatedWhatsAppSentToday: %v", err)
	}
	wantDelta := 2 // only the two automation/ai + sent + today rows
	if got != before+wantDelta {
		t.Errorf("CountAutomatedWhatsAppSentToday = %d, want %d (baseline %d + %d)", got, before+wantDelta, before, wantDelta)
	}
}

// TestCreateJob_TaggedAsAutomation_ForDailyLimitCap covers the "Manual
// Actions" scheduled-job panel: those jobs must be tagged SourceAutomation
// (not SourceStudioUser) so the daily WhatsApp cap in OutboundWorker.dispatch
// blocks them too, the same as real automation/AI sends. A live reply typed
// directly in a conversation (Service.EnqueueReply) is a separate path and
// must stay SourceStudioUser/uncapped — not exercised by this test.
func TestCreateJob_TaggedAsAutomation_ForDailyLimitCap(t *testing.T) {
	repo, pool, studioID, conversationID := setupDailyLimitTestEnv(t)
	ctx := context.Background()
	svc := NewService(repo, NewInProcBus(), "", "")

	jobID, err := svc.CreateJob(ctx, studioID, conversationID, "daily_limit_test manual action", time.Now().UTC().Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DELETE FROM outbound_jobs WHERE id = $1`, jobID); err != nil {
			t.Errorf("cleanup fixture job: %v", err)
		}
	})

	var sourceKind SourceKind
	if err := pool.QueryRow(ctx, `SELECT source_kind FROM outbound_jobs WHERE id = $1`, jobID).Scan(&sourceKind); err != nil {
		t.Fatalf("query created job: %v", err)
	}
	if sourceKind != SourceAutomation {
		t.Errorf("CreateJob source_kind = %q, want %q (so it's covered by the daily WhatsApp cap)", sourceKind, SourceAutomation)
	}
}
