package messaging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// trialPaymentTestEnv mirrors dndTestEnv's setup (real DB, a studio with an
// active whatsapp_meta channel so CreateConversation succeeds) but doesn't
// need the AIWorker — SendTrialPaymentLink and EnqueueReply are plain
// Service methods.
type trialPaymentTestEnv struct {
	pool        *pgxpool.Pool
	msgRepo     *Repo
	msgSvc      *Service
	studioID    uuid.UUID
	channelKind ChannelKind
}

func setupTrialPaymentTestEnv(t *testing.T) *trialPaymentTestEnv {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
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

	var studioID uuid.UUID
	var kindStr string
	// Any active channel works — CreateConversation just needs one to
	// attach the test conversation to; this doesn't exercise the channel
	// adapter at all (SendTrialPaymentLink/EnqueueReply only enqueue).
	err = pool.QueryRow(ctx, `
		SELECT c.studio_id, c.kind
		FROM channel_accounts c
		JOIN campaigns camp ON camp.studio_id = c.studio_id
		WHERE c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &kindStr)
	if err != nil {
		t.Skip("Skipping test; no studio with both an active channel and a campaign found in DB")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	msgRepo := NewRepo(pool, cipher)
	msgSvc := NewService(msgRepo, NewInProcBus(), "", "", nil, nil, "")

	return &trialPaymentTestEnv{pool: pool, msgRepo: msgRepo, msgSvc: msgSvc, studioID: studioID, channelKind: ChannelKind(kindStr)}
}

// setTrialPlanActive sets is_active on the studio's plan literally named
// "Trial" — the same convention IsTrialPlanActive/ResolveTrialAmountSGD use
// to gate trial payment collection — restoring its original value on
// cleanup. Skips the test if the studio has no such plan.
func (e *trialPaymentTestEnv) setTrialPlanActive(t *testing.T, active bool) {
	t.Helper()
	ctx := context.Background()
	var planID uuid.UUID
	var original bool
	if err := e.pool.QueryRow(ctx, `SELECT id, is_active FROM plans WHERE studio_id = $1 AND plan_name = 'Trial' LIMIT 1`, e.studioID).Scan(&planID, &original); err != nil {
		t.Skip("Skipping test; no Trial plan found for studio")
	}
	if _, err := e.pool.Exec(ctx, `UPDATE plans SET is_active = $2 WHERE id = $1`, planID, active); err != nil {
		t.Fatalf("set trial plan active: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `UPDATE plans SET is_active = $2 WHERE id = $1`, planID, original)
	})
}

func (e *trialPaymentTestEnv) seedLeadAndConversation(t *testing.T) (uuid.UUID, *Conversation) {
	t.Helper()
	ctx := context.Background()

	var campaignID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM campaigns WHERE studio_id = $1 LIMIT 1`, e.studioID).Scan(&campaignID); err != nil {
		t.Skip("Skipping test; no campaign found for studio")
	}

	phone := fmt.Sprintf("15556%06d", time.Now().UnixNano()%1000000)
	var leadID uuid.UUID
	err := e.pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status)
		VALUES ($1, $2, 'Trial Payment Toggle Test Lead', 'TrialToggle', 'Test', $3, $4, 'General', 'test', 'new')
		RETURNING id
	`, e.studioID, campaignID, phone+"@example.com", phone).Scan(&leadID)
	if err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID)
	})

	conv, err := e.msgSvc.CreateConversation(ctx, e.studioID, CreateConversationInput{
		ChannelKind:  e.channelKind,
		ContactValue: phone,
		DisplayName:  "Trial Payment Toggle Test Lead",
		LeadID:       &leadID,
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return leadID, conv
}

func (e *trialPaymentTestEnv) needsManualFollowup(t *testing.T, leadID uuid.UUID) bool {
	t.Helper()
	var v bool
	if err := e.pool.QueryRow(context.Background(), `SELECT needs_manual_followup FROM leads WHERE id = $1`, leadID).Scan(&v); err != nil {
		t.Fatalf("read needs_manual_followup: %v", err)
	}
	return v
}

// TestSendTrialPaymentLink_DisabledSkipsPaymentAndFlagsLead confirms that
// when a studio's "Trial" plan is set inactive, SendTrialPaymentLink sends a
// holding message (no Stripe/trigger link) and flags the lead — and that a
// human reply via EnqueueReply clears the flag again.
func TestSendTrialPaymentLink_DisabledSkipsPaymentAndFlagsLead(t *testing.T) {
	env := setupTrialPaymentTestEnv(t)
	env.setTrialPlanActive(t, false)
	leadID, conv := env.seedLeadAndConversation(t)
	ctx := context.Background()

	if env.needsManualFollowup(t, leadID) {
		t.Fatal("lead should not start out flagged")
	}

	body, err := env.msgSvc.SendTrialPaymentLink(ctx, env.studioID, conv.ID, &leadID, "Toggle")
	if err != nil {
		t.Fatalf("SendTrialPaymentLink: %v", err)
	}
	if strings.Contains(body, "http") {
		t.Errorf("expected no payment link in the holding message, got: %q", body)
	}
	if !strings.Contains(strings.ToLower(body), "reach out") {
		t.Errorf("expected a holding message mentioning the team reaching out, got: %q", body)
	}
	if !env.needsManualFollowup(t, leadID) {
		t.Error("lead should be flagged needs_manual_followup after the disabled-payment path")
	}

	var escalatedAt *time.Time
	var aiEnabled bool
	if err := env.pool.QueryRow(ctx, `SELECT escalated_at, ai_enabled FROM conversations WHERE id = $1`, conv.ID).Scan(&escalatedAt, &aiEnabled); err != nil {
		t.Fatalf("read conversation escalation state: %v", err)
	}
	if escalatedAt == nil {
		t.Error("conversation should be escalated (visible in the Inbox's Escalation tab) after the disabled-payment path")
	}
	if aiEnabled {
		t.Error("AI auto-reply should be disabled on an escalated conversation")
	}

	// A human (studio_user) reply should clear the flag. UserID must be a
	// real row (outbound_jobs.source_user_id has a FK to users) — grab
	// whichever user is attached to this studio.
	var studioUserID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM users WHERE studio_id = $1 LIMIT 1`, env.studioID).Scan(&studioUserID); err != nil {
		t.Fatalf("find a studio user for EnqueueReply: %v", err)
	}
	if _, err := env.msgSvc.EnqueueReply(ctx, SendInput{
		StudioID:       env.studioID,
		ConversationID: conv.ID,
		UserID:         studioUserID,
		Body:           "Hey! Following up on your trial — when works for you?",
	}); err != nil {
		t.Fatalf("EnqueueReply: %v", err)
	}
	if env.needsManualFollowup(t, leadID) {
		t.Error("lead should no longer be flagged after a studio_user reply")
	}

	// The Inbox escalation itself is a separate, deliberate action — a
	// reply alone shouldn't silently re-enable AI auto-reply or pull the
	// conversation out of the Escalation tab. Only the explicit
	// resolve-escalation endpoint (internal/messaging/http.go's
	// resolveConversationEscalation) does that.
	if err := env.pool.QueryRow(ctx, `SELECT escalated_at, ai_enabled FROM conversations WHERE id = $1`, conv.ID).Scan(&escalatedAt, &aiEnabled); err != nil {
		t.Fatalf("read conversation escalation state after reply: %v", err)
	}
	if escalatedAt == nil {
		t.Error("conversation should still be escalated after a reply — only explicit resolve-escalation clears it")
	}
	if aiEnabled {
		t.Error("AI auto-reply should still be off after a reply — only explicit resolve-escalation restores it")
	}

	// Re-flag the lead (as if the reply above never happened) and exercise
	// exactly what resolveConversationEscalation (internal/messaging/
	// http.go) does: look up the conversation's lead, clear its
	// needs_manual_followup flag, then resolve the escalation. Verifies
	// both signals close out together from that one action.
	if err := env.msgRepo.SetLeadNeedsManualFollowup(ctx, leadID, true); err != nil {
		t.Fatalf("re-flag lead: %v", err)
	}
	resolvedConv, err := env.msgSvc.GetConversation(ctx, env.studioID, conv.ID)
	if err != nil {
		t.Fatalf("get conversation for resolve: %v", err)
	}
	if resolvedConv.LeadID == nil || *resolvedConv.LeadID != leadID {
		t.Fatalf("conversation.LeadID = %v, want %v", resolvedConv.LeadID, leadID)
	}
	if err := env.msgRepo.SetLeadNeedsManualFollowup(ctx, *resolvedConv.LeadID, false); err != nil {
		t.Fatalf("clear lead flag on resolve: %v", err)
	}
	if err := env.msgRepo.ResolveConversationEscalation(ctx, env.studioID, conv.ID); err != nil {
		t.Fatalf("resolve escalation: %v", err)
	}
	if env.needsManualFollowup(t, leadID) {
		t.Error("lead should no longer be flagged after resolving the escalation")
	}
	if err := env.pool.QueryRow(ctx, `SELECT escalated_at, ai_enabled FROM conversations WHERE id = $1`, conv.ID).Scan(&escalatedAt, &aiEnabled); err != nil {
		t.Fatalf("read conversation escalation state after resolve: %v", err)
	}
	if escalatedAt != nil {
		t.Error("conversation should no longer be escalated after resolve-escalation")
	}
	if !aiEnabled {
		t.Error("AI auto-reply should be restored after resolve-escalation")
	}
}

// TestSendTrialPaymentLink_EnabledSendsPaymentLink is the control case: with
// the Trial plan active (the default), the lead is never flagged.
func TestSendTrialPaymentLink_EnabledSendsPaymentLink(t *testing.T) {
	env := setupTrialPaymentTestEnv(t)
	env.setTrialPlanActive(t, true)
	leadID, conv := env.seedLeadAndConversation(t)
	ctx := context.Background()

	if _, err := env.msgSvc.SendTrialPaymentLink(ctx, env.studioID, conv.ID, &leadID, "Toggle"); err != nil {
		t.Fatalf("SendTrialPaymentLink: %v", err)
	}
	if env.needsManualFollowup(t, leadID) {
		t.Error("lead should not be flagged when trial payment collection is enabled")
	}
}
