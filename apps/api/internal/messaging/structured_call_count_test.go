package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
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

// fakeStructuredCompleter is a deterministic, call-counting stand-in for
// a live provider call, injected via llmgateway.NewForTest — the same
// technique internal/integrations/llmgateway's own gateway_test.go uses
// internally, exposed for this cross-package use rather than duplicating
// Client's unexported fields here.
type fakeStructuredCompleter struct {
	mu    sync.Mutex
	calls int
	reply string
}

func (f *fakeStructuredCompleter) Complete(ctx context.Context, req bifrost.Request) (bifrost.Reply, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return bifrost.Reply{Text: f.reply}, nil
}

func (f *fakeStructuredCompleter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// setupStructuredCallCountTest seeds a studio/lead/contact/conversation/
// inbound message in the real host-test DB (same convention every other
// handleMessage integration test in this package follows) and returns a
// worker wired to the given fake completer via llmgateway.NewForTest, so
// handleMessage's real production path runs end-to-end with no live
// network call and a reply fully controlled by the test.
func setupStructuredCallCountTest(t *testing.T, completer *fakeStructuredCompleter) (ctx context.Context, pool *pgxpool.Pool, worker *AIWorker, studioID, convID, msgID uuid.UUID) {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" || os.Getenv("TOKEN_ENCRYPTION_KEY") == "" {
		t.Skip("skipping integration test; no DB/cipher env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	ctx = context.Background()
	var err error
	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	var channelID, campaignID uuid.UUID
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

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	studiosRepo := studios.NewRepo(pool, cipher)

	// Seed a "groq" platform key — the customer_reply route's first
	// provider — so llmgateway.CompleteStructured actually attempts a
	// call (routed to the fake) instead of skipping straight to "no
	// provider configured". Deliberately NOT seeding a "gemini" key: that
	// gates ai_worker.go's retrieval block, which this test isn't
	// exercising and doesn't need (no knowledge-base lookup involved).
	var hadGroqRow bool
	var origKeyCiphertext, origDataKeyCiphertext, origFingerprint, origStatus string
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = 'groq'`).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origFingerprint, &origStatus); err == nil {
		hadGroqRow = true
	}
	if err := studiosRepo.SetPlatformAIProviderKey(ctx, "groq", "test-key-groq", nil); err != nil {
		t.Fatalf("seed groq key: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if hadGroqRow {
			_, _ = pool.Exec(ctx, `UPDATE platform_ai_keys SET key_ciphertext=$1, data_key_ciphertext=$2, key_fingerprint=$3, status=$4 WHERE provider='groq'`,
				origKeyCiphertext, origDataKeyCiphertext, origFingerprint, origStatus)
		} else {
			_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = 'groq'`)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = 'groq'`)
	})

	phone := "659" + uuid.New().String()[:7]
	var leadID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status, auto_contact_stage)
		VALUES ($1, $2, 'Structured Call Count Test Lead', 'StructuredCallCount', 'Test', $3, $4, 'General', 'test', 'new', 'completed')
		RETURNING id
	`, studioID, campaignID, phone+"@example.com", phone).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID) })

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone',$3,'Structured Call Count Test Lead')`,
		contactID, studioID, phone); err != nil {
		t.Fatalf("create test contact: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM contact_identities WHERE id = $1`, contactID) })

	convID = uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, lead_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,$5,'structured-call-count-test-thread',true)`,
		convID, studioID, channelID, contactID, leadID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE id = $1`, convID) })

	msgRows, err := pool.Query(ctx, `
		INSERT INTO messages (conversation_id, studio_id, direction, source_kind, body, external_id, status, sent_at)
		VALUES ($1, $2, 'inbound', 'customer', 'Hey, just wanted to say thanks!', $3, 'sent', now())
		RETURNING id
	`, convID, studioID, "structured-call-count-test-"+uuid.New().String())
	if err != nil {
		t.Fatalf("insert inbound message: %v", err)
	}
	if msgRows.Next() {
		_ = msgRows.Scan(&msgID)
	}
	msgRows.Close()
	if msgID == uuid.Nil {
		t.Fatalf("insert inbound message: no id returned")
	}

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "", nil, nil, "")
	leadsRepo := leads.NewRepo(pool, pool)
	llmRepo := llm.NewRepo(pool, cipher)
	gwRepo := llmgateway.NewRepo(pool)
	llmGateway := llmgateway.NewForTest(completer, llmRepo, studiosRepo, gwRepo, slog.Default())

	worker = NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, nil, nil, nil, llmRepo, llmGateway, slog.Default())
	return ctx, pool, worker, studioID, convID, msgID
}

// TestHandleMessage_GeneratesReply_MakesAtMostOneCustomerReplyLLMCall is
// the regression test for the ai-reply-flow-llm-design structured-reply
// spec's core requirement: "exactly one LLM call ... for intent/
// expansion/rerank/reply purposes combined". Before the collapse this
// test was written for, handleMessage fired separate ClassifyIntent/
// ExpandQuery/RerankChunks calls ahead of the structured reply call; this
// proves the real production path now makes exactly one completion call
// to generate a reply.
func TestHandleMessage_GeneratesReply_MakesAtMostOneCustomerReplyLLMCall(t *testing.T) {
	completer := &fakeStructuredCompleter{
		reply: `{"reply":"Happy to help anytime!","action":"answer","language":"en","intent":"thanks","sentiment":"positive","facts_used":[],"tool_calls":[]}`,
	}
	ctx, pool, worker, studioID, convID, msgID := setupStructuredCallCountTest(t, completer)

	if err := worker.handleMessage(ctx, studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	if got := completer.callCount(); got != 1 {
		t.Errorf("completer.callCount() = %d, want exactly 1 LLM call for the generated reply", got)
	}

	var outboundCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbound_jobs WHERE conversation_id = $1 AND source_kind = 'ai'`, convID).Scan(&outboundCount); err != nil {
		t.Fatalf("query outbound_jobs: %v", err)
	}
	if outboundCount == 0 {
		t.Error("no AI reply was queued for the customer")
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM outbound_jobs WHERE conversation_id = $1`, convID) })
}

// TestHandleMessage_StructuredHandoffAction_EscalatesWithoutSendingReply
// covers the new handoff path this collapse introduced: with the old
// pre-reply ClassifyIntent call removed, a complaint/sensitive-request
// signal the free keyword checks don't already catch can only be acted
// on via the structured call's own "handoff" action. This proves that
// action escalates the conversation to a human instead of sending
// structured.Reply, still within the same single LLM call.
func TestHandleMessage_StructuredHandoffAction_EscalatesWithoutSendingReply(t *testing.T) {
	completer := &fakeStructuredCompleter{
		reply: `{"reply":"","action":"handoff","language":"en","intent":"complaint","sentiment":"negative","facts_used":[],"tool_calls":[]}`,
	}
	ctx, pool, worker, studioID, convID, msgID := setupStructuredCallCountTest(t, completer)

	if err := worker.handleMessage(ctx, studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	if got := completer.callCount(); got != 1 {
		t.Errorf("completer.callCount() = %d, want exactly 1 LLM call before escalating", got)
	}

	var escalatedAtSet bool
	var reason string
	if err := pool.QueryRow(ctx, `SELECT escalated_at IS NOT NULL, escalated_reason FROM conversations WHERE id = $1`, convID).
		Scan(&escalatedAtSet, &reason); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if !escalatedAtSet {
		t.Error("escalated_at was not set — a structured handoff action must escalate to staff")
	}

	var handoffOutboundCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM outbound_jobs WHERE conversation_id = $1 AND source_ref = 'ai_requested_handoff'
	`, convID).Scan(&handoffOutboundCount); err != nil {
		t.Fatalf("query outbound_jobs: %v", err)
	}
	if handoffOutboundCount == 0 {
		t.Error("no handoff holding message was queued for the customer")
	}

	var structuredReplySentCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM outbound_jobs WHERE conversation_id = $1 AND body = ''
	`, convID).Scan(&structuredReplySentCount); err != nil {
		t.Fatalf("query outbound_jobs: %v", err)
	}
	if structuredReplySentCount != 0 {
		t.Error("the structured call's own (empty) reply text must never be sent on a handoff action")
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM outbound_jobs WHERE conversation_id = $1`, convID) })
}
