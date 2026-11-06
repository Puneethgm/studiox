package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

// TestHandleMessage_AllProvidersFail_EscalatesWithHandoff is the "no silent
// drops" regression test: when every LLM provider fails or is unconfigured
// (here, llmGateway is nil, which llmgateway.Client.Complete's nil-receiver
// guard treats as an immediate "no reply" — a real stand-in for "nothing
// answered"), the customer must not simply be left unanswered. The
// conversation should be escalated, a holding message queued, and an alert
// recorded — see openspec/changes/llm-gateway-and-key-management, task 1.3.
func TestHandleMessage_AllProvidersFail_EscalatesWithHandoff(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	var studioID, channelID, campaignID uuid.UUID
	err = pool.QueryRow(ctx, `
		SELECT c.studio_id, c.id, camp.id
		FROM channel_accounts c
		JOIN campaigns camp ON camp.studio_id = c.studio_id
		WHERE c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &channelID, &campaignID)
	if err != nil {
		t.Skip("skipping test; no studio with both an active channel and a campaign found in DB")
	}

	// Force the platform AI switch on for this studio, restoring it after.
	var origAIAccessEnabled bool
	if err := pool.QueryRow(ctx, `SELECT ai_access_enabled FROM studios WHERE id = $1`, studioID).Scan(&origAIAccessEnabled); err != nil {
		t.Fatalf("read original ai_access_enabled: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE studios SET ai_access_enabled = true WHERE id = $1`, studioID); err != nil {
		t.Fatalf("enable ai access: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE studios SET ai_access_enabled = $2 WHERE id = $1`, studioID, origAIAccessEnabled)
	})

	phone := "659" + uuid.New().String()[:7]
	var leadID uuid.UUID
	// auto_contact_stage = 'completed': any other mid-flow stage (e.g. the
	// default 'initial') makes handleMessage treat the bot's scripted
	// auto-contact flow as "owning" this lead and return before ever
	// reaching the free-form AI-reply waterfall this test targets.
	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status, auto_contact_stage)
		VALUES ($1, $2, 'All Providers Failed Test Lead', 'AllProvidersFailed', 'Test', $3, $4, 'General', 'test', 'new', 'completed')
		RETURNING id
	`, studioID, campaignID, phone+"@example.com", phone).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID) })

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone',$3,'All Providers Failed Test Lead')`,
		contactID, studioID, phone); err != nil {
		t.Fatalf("create test contact: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM contact_identities WHERE id = $1`, contactID) })

	convID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, lead_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,$5,'all-providers-failed-test-thread',true)`,
		convID, studioID, channelID, contactID, leadID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE id = $1`, convID) })

	// A plain, unremarkable statement — not a question (no "?" and no
	// question word), so it skips the deterministic kb-coverage pre-check
	// (kbCoversQuestion is permissive for non-questions) as well as the
	// stop-keyword, complaint, class-action, trial-keyword and menu-choice
	// special cases, reaching the LLM waterfall itself.
	msg, err := pool.Query(ctx, `
		INSERT INTO messages (conversation_id, studio_id, direction, source_kind, body, external_id, status, sent_at)
		VALUES ($1, $2, 'inbound', 'customer', 'Hey, just wanted to say thanks!', $3, 'sent', now())
		RETURNING id
	`, convID, studioID, "all-providers-failed-test-"+uuid.New().String())
	if err != nil {
		t.Fatalf("insert inbound message: %v", err)
	}
	var msgID uuid.UUID
	if msg.Next() {
		_ = msg.Scan(&msgID)
	}
	msg.Close()
	if msgID == uuid.Nil {
		t.Fatalf("insert inbound message: no id returned")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "", nil, nil, "")
	studiosRepo := studios.NewRepo(pool, cipher)
	leadsRepo := leads.NewRepo(pool, pool)
	llmRepo := llm.NewRepo(pool, cipher)

	// bifrostClient and llmGateway: both nil — llmGateway.Complete's
	// nil-receiver guard makes every call a guaranteed no-op, i.e. a
	// deterministic "all providers failed" run with no network calls.
	worker := NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, nil, nil, nil, llmRepo, nil, slog.Default())

	if err := worker.handleMessage(ctx, studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	var escalatedAtSet bool
	var reason string
	var aiEnabled bool
	if err := pool.QueryRow(ctx, `SELECT escalated_at IS NOT NULL, escalated_reason, ai_enabled FROM conversations WHERE id = $1`, convID).
		Scan(&escalatedAtSet, &reason, &aiEnabled); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if !escalatedAtSet {
		t.Error("escalated_at was not set — an all-providers-failed reply must hand off to staff, not drop silently")
	}
	if reason != "All AI providers failed to generate a reply" {
		t.Errorf("escalated_reason = %q, want %q", reason, "All AI providers failed to generate a reply")
	}
	if aiEnabled {
		t.Error("ai_enabled is still true after escalation")
	}

	var outboundCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM outbound_jobs
		WHERE conversation_id = $1 AND source_ref = 'all_providers_failed_handoff'
	`, convID).Scan(&outboundCount); err != nil {
		t.Fatalf("query outbound_jobs: %v", err)
	}
	if outboundCount == 0 {
		t.Error("no holding message was queued for the customer")
	}

	var alertCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM platform_alerts WHERE kind = 'all_providers_failed' AND studio_id = $1
	`, studioID).Scan(&alertCount); err != nil {
		t.Fatalf("query platform_alerts: %v", err)
	}
	if alertCount == 0 {
		t.Error("no platform_alerts row was recorded for the all-providers-failed condition")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_alerts WHERE kind = 'all_providers_failed' AND studio_id = $1`, studioID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbound_jobs WHERE conversation_id = $1`, convID)
	})
}
