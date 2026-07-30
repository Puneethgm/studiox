package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/leads"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

func TestIsStopMessage(t *testing.T) {
	cases := map[string]bool{
		"stop":                     true,
		"Stop":                     true,
		"STOP":                     true,
		" stop ":                   true,
		"stop.":                    true,
		"stop!":                    true,
		"unsubscribe":              true,
		"opt out":                  true,
		"optout":                   true,
		"stop by later":            false,
		"please stop messaging me": false,
		"":                         false,
		"hi":                       false,
		"cancel":                   false, // deliberately not a stop keyword — too ambiguous
	}
	for in, want := range cases {
		if got := isStopMessage(in); got != want {
			t.Errorf("isStopMessage(%q) = %v, want %v", in, got, want)
		}
	}
}

// dndTestEnv bundles everything an AIWorker.handleMessage call needs,
// connected to a real DB using the same .env-driven pattern as the other
// integration tests in this package.
type dndTestEnv struct {
	pool      *pgxpool.Pool
	leadsRepo *leads.Repo
	msgRepo   *Repo
	msgSvc    *Service
	worker    *AIWorker
	studioID  uuid.UUID
	externID  string
}

func setupDNDTestEnv(t *testing.T) *dndTestEnv {
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

	var studioID uuid.UUID
	var externalID string
	err = pool.QueryRow(ctx, `
		SELECT s.id, c.external_id
		FROM channel_accounts c
		JOIN studios s ON s.id = c.studio_id
		WHERE c.kind = 'whatsapp_meta' AND c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &externalID)
	if err != nil {
		t.Skip("Skipping test; no active WhatsApp channel found in DB")
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}

	leadsRepo := leads.NewRepo(pool)
	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "")
	studiosRepo := studios.NewRepo(pool, cipher)

	worker := NewAIWorker(msgBus, msgRepo, msgSvc, studiosRepo, leadsRepo, nil, nil, slog.Default())

	return &dndTestEnv{
		pool:      pool,
		leadsRepo: leadsRepo,
		msgRepo:   msgRepo,
		msgSvc:    msgSvc,
		worker:    worker,
		studioID:  studioID,
		externID:  externalID,
	}
}

// seedLeadAndConversation creates a lead (bypassing outbox/autocontact) plus
// a linked conversation over the studio's active whatsapp_meta channel, for
// isolated handleMessage testing.
func (e *dndTestEnv) seedLeadAndConversation(t *testing.T, email string) (*leads.Lead, *Conversation) {
	t.Helper()
	ctx := context.Background()

	var campaignID uuid.UUID
	err := e.pool.QueryRow(ctx, `SELECT id FROM campaigns WHERE studio_id = $1 LIMIT 1`, e.studioID).Scan(&campaignID)
	if err != nil {
		t.Skip("Skipping test; no campaign found for studio")
	}

	phone := fmt.Sprintf("15555%06d", time.Now().UnixNano()%1000000)

	var leadID uuid.UUID
	err = e.pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status)
		VALUES ($1, $2, 'DND AI Test Lead', 'DND', 'AITest', $3, $4, 'General', 'test', 'new')
		RETURNING id
	`, e.studioID, campaignID, email, phone).Scan(&leadID)
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID)
	})

	conv, err := e.msgSvc.CreateConversation(ctx, e.studioID, CreateConversationInput{
		ChannelKind:  KindWhatsAppMeta,
		ContactValue: phone,
		DisplayName:  "DND AI Test Lead",
		LeadID:       &leadID,
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	lead, err := e.leadsRepo.GetLead(ctx, e.studioID, leadID)
	if err != nil {
		t.Fatalf("get seeded lead: %v", err)
	}
	return lead, conv
}

// insertInboundMessage inserts a raw inbound customer message directly
// (bypassing the webhook parsing layer) so handleMessage can be invoked
// deterministically without depending on the async event bus.
func (e *dndTestEnv) insertInboundMessage(t *testing.T, conv *Conversation, body string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	msg, err := e.msgRepo.InsertMessage(ctx, tx, CreateMessageInput{
		ConversationID: conv.ID,
		StudioID:       e.studioID,
		Direction:      DirectionInbound,
		SourceKind:     SourceCustomer,
		Body:           body,
		ExternalID:     "dnd-test-" + uuid.New().String(),
		Status:         MsgSent,
		SentAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if msg == nil {
		t.Fatalf("insert message: got nil (unexpected dedupe)")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	return msg.ID
}

func TestHandleMessage_StopKeyword_EnablesDNDAndCancelsPendingJobs(t *testing.T) {
	env := setupDNDTestEnv(t)
	ctx := context.Background()

	lead, conv := env.seedLeadAndConversation(t, "dnd-ai-stop-test@example.com")
	if lead.DNDEnabled {
		t.Fatalf("precondition failed: seeded lead already has DND enabled")
	}

	// Queue a pending follow-up the same way autocontact would.
	if _, err := env.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       env.studioID,
		ConversationID: conv.ID,
		Body:           "Just following up — {{contact.first_name}}",
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:followup:1", lead.ID),
		ScheduledFor:   time.Now().UTC().Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("enqueue pending follow-up: %v", err)
	}

	msgID := env.insertInboundMessage(t, conv, "stop")

	if err := env.worker.handleMessage(ctx, env.studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	updated, err := env.leadsRepo.GetLead(ctx, env.studioID, lead.ID)
	if err != nil {
		t.Fatalf("GetLead after stop: %v", err)
	}
	if !updated.DNDEnabled {
		t.Errorf("DNDEnabled after 'stop' = false; want true")
	}
	if updated.Status != lead.Status {
		t.Errorf("Status changed by 'stop' = %q; want unchanged %q", updated.Status, lead.Status)
	}

	var pendingCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbound_jobs WHERE conversation_id = $1 AND status = 'pending'
	`, conv.ID).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending jobs: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("pending outbound jobs after 'stop' = %d; want 0", pendingCount)
	}
}

func TestHandleMessage_AlreadyDND_SuppressesReplyWithoutStopKeyword(t *testing.T) {
	env := setupDNDTestEnv(t)
	ctx := context.Background()

	lead, conv := env.seedLeadAndConversation(t, "dnd-ai-manual-test@example.com")

	// Manually enable DND (simulating the inbox toggle), independent of the
	// "stop" keyword.
	if err := env.leadsRepo.SetDNDEnabled(ctx, env.studioID, lead.ID, true); err != nil {
		t.Fatalf("SetDNDEnabled: %v", err)
	}

	if _, err := env.msgRepo.EnqueueOutbound(ctx, OutboundJob{
		StudioID:       env.studioID,
		ConversationID: conv.ID,
		Body:           "Should never send",
		SourceKind:     SourceAutomation,
		SourceRef:      fmt.Sprintf("lead:%s:followup:1", lead.ID),
		ScheduledFor:   time.Now().UTC().Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("enqueue pending follow-up: %v", err)
	}

	// An ordinary message — NOT a stop keyword — should still be suppressed
	// because the lead already has DND enabled.
	msgID := env.insertInboundMessage(t, conv, "hello are you still open on weekends?")

	if err := env.worker.handleMessage(ctx, env.studioID, msgID); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	// No automated reply should have been enqueued for this inbound message.
	var replyCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbound_jobs
		WHERE conversation_id = $1 AND source_kind = 'automation' AND created_at >= (
			SELECT created_at FROM messages WHERE id = $2
		)
	`, conv.ID, msgID).Scan(&replyCount); err != nil {
		t.Fatalf("count replies: %v", err)
	}
	if replyCount != 0 {
		t.Errorf("automated replies sent to DND lead = %d; want 0", replyCount)
	}

	// The pre-existing pending follow-up must also have been cancelled.
	var pendingCount int
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbound_jobs WHERE conversation_id = $1 AND status = 'pending'
	`, conv.ID).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending jobs: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("pending outbound jobs for DND lead = %d; want 0", pendingCount)
	}
}
