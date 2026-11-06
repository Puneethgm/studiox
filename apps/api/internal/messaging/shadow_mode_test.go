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

	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/integrations/llmgateway"
	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

// TestHandleMessage_ShadowMode_RunsPipelineButNeverSends is task 9.2's
// real, end-to-end verification. No genuinely live customer traffic
// exists in this sandbox to passively observe (checked: the one
// configured WhatsApp Web channel is idle, last real message Oct 28 —
// see this task's completion note in tasks.md for the full honesty
// note), so this injects a real-shaped inbound message into a real
// studio/conversation/lead in the host-test DB, with ai_shadow_mode
// enabled on that studio, and drives it through handleMessage — the
// exact same core pipeline function every other "live test" in this
// change (e.g. TestHandleMessage_AllProvidersFail_EscalatesWithHandoff,
// the guard-repair tests in guards_integration_test.go) uses as its
// fidelity bar, with a REAL llmGateway making REAL provider calls, not a
// stub. It asserts the three things that make this "shadow" rather than
// "live": an ai_turns row is written with outcome='shadow', no
// outbound_jobs row is created, and the conversation is not escalated.
func TestHandleMessage_ShadowMode_RunsPipelineButNeverSends(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" || os.Getenv("TOKEN_ENCRYPTION_KEY") == "" {
		t.Skip("skipping integration test; no DB/cipher env vars found")
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

	bf := bifrost.New()
	if bf == nil {
		t.Skip("bifrost not configured (BIFROST_ENABLED=false) — shadow mode's real-LLM path can't be exercised")
	}

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

	// Force the platform AI switch on and shadow mode on for this studio,
	// restoring both after.
	var origAIAccessEnabled, origShadowMode bool
	if err := pool.QueryRow(ctx, `SELECT ai_access_enabled, ai_shadow_mode FROM studios WHERE id = $1`, studioID).
		Scan(&origAIAccessEnabled, &origShadowMode); err != nil {
		t.Fatalf("read original studio flags: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE studios SET ai_access_enabled = true, ai_shadow_mode = true WHERE id = $1`, studioID); err != nil {
		t.Fatalf("enable ai access + shadow mode: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE studios SET ai_access_enabled = $2, ai_shadow_mode = $3 WHERE id = $1`,
			studioID, origAIAccessEnabled, origShadowMode)
	})

	phone := "659" + uuid.New().String()[:7]
	var leadID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status, auto_contact_stage)
		VALUES ($1, $2, 'Shadow Mode Test Lead', 'ShadowMode', 'Test', $3, $4, 'General', 'test', 'new', 'completed')
		RETURNING id
	`, studioID, campaignID, phone+"@example.com", phone).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID) })

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone',$3,'Shadow Mode Test Lead')`,
		contactID, studioID, phone); err != nil {
		t.Fatalf("create test contact: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM contact_identities WHERE id = $1`, contactID) })

	convID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, lead_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,$5,'shadow-mode-test-thread',true)`,
		convID, studioID, channelID, contactID, leadID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE id = $1`, convID) })
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM ai_turns WHERE conversation_id = $1`, convID) })

	// A real-shaped customer message — same style/wording a genuine
	// WA-Web inbound webhook payload would carry, inserted exactly the
	// way the real inbound path inserts it, not a synthetic test string.
	// A plain, unremarkable statement (not a question) — same choice as
	// TestHandleMessage_AllProvidersFail_EscalatesWithHandoff, for the
	// same reason: it skips the pre-existing, deterministic kb-coverage
	// hard gate (which predates and is unrelated to this change — see
	// this test's package comment) and reaches the actual new pipeline
	// this test exists to validate.
	var msgID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, studio_id, direction, source_kind, body, external_id, status, sent_at)
		VALUES ($1, $2, 'inbound', 'customer', 'Hey, just wanted to say thanks!', $3, 'sent', now())
		RETURNING id
	`, convID, studioID, "shadow-mode-test-"+uuid.New().String()).Scan(&msgID); err != nil {
		t.Fatalf("insert inbound message: %v", err)
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
	gwRepo := llmgateway.NewRepo(pool)
	gw := llmgateway.New(bf, llmRepo, studiosRepo, gwRepo, slog.Default())

	worker := NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, nil, nil, nil, llmRepo, gw, slog.Default())

	if err := worker.handleMessage(ctx, studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	// 1. An ai_turns row was written with outcome='shadow'.
	var turnCount int
	var guardOutcome string
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM ai_turns WHERE conversation_id = $1 AND outcome = 'shadow'
	`, convID).Scan(&turnCount); err != nil {
		t.Fatalf("query ai_turns: %v", err)
	}
	if turnCount == 0 {
		// The real call may have failed guards/providers instead of
		// reaching a clean "shadow" outcome — check what actually
		// happened before failing, since any outcome still proves the
		// pipeline ran; only "a live send occurred" is the real failure.
		var anyOutcome string
		_ = pool.QueryRow(ctx, `SELECT outcome FROM ai_turns WHERE conversation_id = $1 ORDER BY created_at DESC LIMIT 1`, convID).Scan(&anyOutcome)
		t.Fatalf("no ai_turns row with outcome='shadow' was written (most recent turn outcome seen: %q) — shadow mode did not record the run", anyOutcome)
	}
	_ = pool.QueryRow(ctx, `SELECT guard_outcome FROM ai_turns WHERE conversation_id = $1 AND outcome = 'shadow' LIMIT 1`, convID).Scan(&guardOutcome)
	t.Logf("shadow turn recorded, guard_outcome=%q", guardOutcome)

	// 2. No outbound reply was ever enqueued for this conversation.
	var outboundCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbound_jobs WHERE conversation_id = $1`, convID).Scan(&outboundCount); err != nil {
		t.Fatalf("query outbound_jobs: %v", err)
	}
	if outboundCount != 0 {
		t.Errorf("outbound_jobs count = %d, want 0 — shadow mode must never send a reply to the customer", outboundCount)
	}

	// 3. The conversation was not escalated — shadow mode must not take
	// any live action, not even an escalation handoff.
	var escalatedAtSet bool
	if err := pool.QueryRow(ctx, `SELECT escalated_at IS NOT NULL FROM conversations WHERE id = $1`, convID).Scan(&escalatedAtSet); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if escalatedAtSet {
		t.Error("conversation was escalated — shadow mode must not take any live action on the conversation")
	}
}
