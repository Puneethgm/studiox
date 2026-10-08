package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// setupPlanCheckoutTest picks an existing studio with an active channel and a
// campaign (same convention as trial_payment_toggle_test.go — leads require a
// real campaign_id, so a from-scratch throwaway studio isn't worth the extra
// fixture code) and temporarily clears its Stripe key — the exact real-world
// case this test is for — restoring it on cleanup. A throwaway lead and
// conversation are created to build a membership checkout against.
func setupPlanCheckoutTest(t *testing.T) (svc *Service, pool *pgxpool.Pool, studioID, convID, leadID uuid.UUID) {
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

	var kindStr string
	var campaignID uuid.UUID
	err = pool.QueryRow(ctx, `
		SELECT c.studio_id, c.kind, camp.id
		FROM channel_accounts c
		JOIN campaigns camp ON camp.studio_id = c.studio_id
		WHERE c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &kindStr, &campaignID)
	if err != nil {
		t.Skip("skipping test; no studio with both an active channel and a campaign found in DB")
	}

	var channelID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM channel_accounts WHERE studio_id = $1 AND status = 'active' LIMIT 1`, studioID).Scan(&channelID); err != nil {
		t.Fatalf("find active channel: %v", err)
	}

	var origStripeKey string
	if err := pool.QueryRow(ctx, `SELECT stripe_secret_key FROM studios WHERE id = $1`, studioID).Scan(&origStripeKey); err != nil {
		t.Fatalf("read original stripe key: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE studios SET stripe_secret_key = '' WHERE id = $1`, studioID); err != nil {
		t.Fatalf("clear stripe key: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE studios SET stripe_secret_key = $2 WHERE id = $1`, studioID, origStripeKey)
	})

	phone := "659" + uuid.New().String()[:7]
	if err := pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status, auto_contact_stage)
		VALUES ($1, $2, 'Plan Checkout Test Lead', 'PlanCheckout', 'Test', $3, $4, 'General', 'test', 'new', 'awaiting_plan_selection')
		RETURNING id
	`, studioID, campaignID, phone+"@example.com", phone).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID) })

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone',$3,'Plan Checkout Test Lead')`,
		contactID, studioID, phone); err != nil {
		t.Fatalf("create test contact: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM contact_identities WHERE id = $1`, contactID)
	})

	conv := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, lead_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,$5,'plan-checkout-test-thread',true)`,
		conv, studioID, channelID, contactID, leadID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE id = $1`, conv) })
	convID = conv

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	msgRepo := NewRepo(pool, cipher)
	svc = NewService(msgRepo, NewInProcBus(), "", "", nil, nil, "")
	return svc, pool, studioID, convID, leadID
}

// The real-world gap this closes: a customer picks a membership plan, but the
// studio never configured Stripe. Before this fix, buildPlanCheckoutBody just
// sent a generic "team will reach out" message and left it at that — exactly
// the kind of silent real-money dead end the trial flow already escalates for,
// but membership didn't.
func TestBuildPlanCheckoutBody_EscalatesWhenStripeNotConfigured(t *testing.T) {
	svc, pool, studioID, convID, leadID := setupPlanCheckoutTest(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	plan := Plan{PlanName: "Pro", PriceSGD: 8900, BillingCycle: "monthly"}
	body := svc.buildPlanCheckoutBody(ctx, tx, studioID, convID, leadID, plan, "")
	if body == "" {
		t.Fatal("expected a non-empty fallback message")
	}

	var escalatedAtSet bool
	var reason string
	var aiEnabled bool
	if err := pool.QueryRow(ctx, `SELECT escalated_at IS NOT NULL, escalated_reason, ai_enabled FROM conversations WHERE id = $1`, convID).
		Scan(&escalatedAtSet, &reason, &aiEnabled); err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	if !escalatedAtSet {
		t.Error("escalated_at was not set — a real membership pick with no Stripe configured would go unanswered")
	}
	if aiEnabled {
		t.Error("ai_enabled is still true after escalation")
	}
	if reason == "" {
		t.Error("escalated_reason is empty — staff would have no idea why this needs attention")
	}

	var needsFollowup bool
	if err := pool.QueryRow(ctx, `SELECT needs_manual_followup FROM leads WHERE id = $1`, leadID).Scan(&needsFollowup); err == nil && !needsFollowup {
		t.Error("lead was not flagged needs_manual_followup")
	}
}
