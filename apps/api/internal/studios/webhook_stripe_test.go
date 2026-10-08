package studios

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// setupEscalationTestConversation connects to the real dev DB and creates a
// throwaway studio + channel + contact + conversation to escalate against, with
// cleanup registered.
func setupEscalationTestConversation(t *testing.T) (pool *pgxpool.Pool, studio *Studio, convID string) {
	t.Helper()
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

	studioID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color, contact_email) VALUES ($1,$2,'Escalation Test','#7c3aed','owner@example.com')`,
		studioID, "escalation-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	channelID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO channel_accounts (id, studio_id, kind, external_id, display_handle, access_token_enc) VALUES ($1,$2,'whatsapp_web','test-channel','Test','enc')`,
		channelID, studioID); err != nil {
		t.Fatalf("create test channel: %v", err)
	}

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone','6591234567','Test Contact')`,
		contactID, studioID); err != nil {
		t.Fatalf("create test contact: %v", err)
	}

	conv := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,'test-thread',true)`,
		conv, studioID, channelID, contactID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}

	s := &Studio{ID: studioID, Name: "Escalation Test", ContactEmail: "owner@example.com"}
	return pool, s, conv.String()
}

// Confirms the real effect: after a trial/membership purchase, the conversation
// actually ends up in the Escalation tab's state (escalated_at set, AI turned off)
// with the plan-name reason — exactly what the customer screenshot was missing.
func TestEscalateConversationForPurchase_MarksConversationEscalated(t *testing.T) {
	pool, studio, convID := setupEscalationTestConversation(t)
	ctx := context.Background()
	h := NewStripeWebhookHandler(NewService(&Repo{pool: pool}, nil, nil, nil, nil, nil, ""), "")

	h.escalateConversationForPurchase(ctx, studio, convID, "Puneeth G M", "917483974512", "Customer purchased a Trial — Plan: Trial")

	var escalatedAtSet bool
	var reason string
	var aiEnabled bool
	if err := pool.QueryRow(ctx, `SELECT escalated_at IS NOT NULL, escalated_reason, ai_enabled FROM conversations WHERE id = $1`, convID).
		Scan(&escalatedAtSet, &reason, &aiEnabled); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if !escalatedAtSet {
		t.Error("escalated_at was not set — conversation would not show in the Escalation tab")
	}
	if reason != "Customer purchased a Trial — Plan: Trial" {
		t.Errorf("escalated_reason = %q, want the plan-name reason", reason)
	}
	if aiEnabled {
		t.Error("ai_enabled is still true — AI would keep auto-replying on an escalated conversation")
	}
}

// A second escalation (e.g. a retried webhook) must not clobber the first one's
// timestamp/reason — only the first call should actually change anything.
func TestEscalateConversationForPurchase_DoesNotReescalate(t *testing.T) {
	pool, studio, convID := setupEscalationTestConversation(t)
	ctx := context.Background()
	h := NewStripeWebhookHandler(NewService(&Repo{pool: pool}, nil, nil, nil, nil, nil, ""), "")

	h.escalateConversationForPurchase(ctx, studio, convID, "A", "1", "first reason")
	var firstAt interface{}
	_ = pool.QueryRow(ctx, `SELECT escalated_at FROM conversations WHERE id = $1`, convID).Scan(&firstAt)

	h.escalateConversationForPurchase(ctx, studio, convID, "A", "1", "second reason — should be ignored")

	var reason string
	var secondAt interface{}
	if err := pool.QueryRow(ctx, `SELECT escalated_reason, escalated_at FROM conversations WHERE id = $1`, convID).Scan(&reason, &secondAt); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if reason != "first reason" {
		t.Errorf("escalated_reason = %q, want it to stay %q (second call should be a no-op)", reason, "first reason")
	}
	if fmt.Sprint(firstAt) != fmt.Sprint(secondAt) {
		t.Errorf("escalated_at changed on the second call: %v -> %v", firstAt, secondAt)
	}
}

// An empty conversation id (no conversation exists yet) must be a safe no-op, not
// a panic or a wasted DB round-trip against a non-existent row.
func TestEscalateConversationForPurchase_EmptyConvIDNoop(t *testing.T) {
	pool, studio, _ := setupEscalationTestConversation(t)
	ctx := context.Background()
	h := NewStripeWebhookHandler(NewService(&Repo{pool: pool}, nil, nil, nil, nil, nil, ""), "")
	h.escalateConversationForPurchase(ctx, studio, "", "A", "1", "reason") // must not panic
}
