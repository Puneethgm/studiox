package messaging

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestGetLatestSubscriptionForLead_And_AlreadyPaidBody covers the shared
// helper used by both the trial flow (SendTrialPaymentLink) and the
// membership flow (processInboundLeadAutomation's isMember/wantsMembership
// branches): once a lead has an actual paid subscription, they should be
// told the specific plan and paid date/time instead of a generic message.
func TestGetLatestSubscriptionForLead_And_AlreadyPaidBody(t *testing.T) {
	env := setupTrialPaymentTestEnv(t)
	ctx := context.Background()
	repo := env.msgRepo

	var planID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM plans WHERE studio_id = $1 LIMIT 1`, env.studioID).Scan(&planID); err != nil {
		t.Skip("Skipping test; no plan found for studio")
	}

	leadID, conv := env.seedLeadAndConversation(t)

	// No subscription yet — should return (nil, nil), not an error.
	if sub, err := repo.GetLatestSubscriptionForLead(ctx, leadID); err != nil || sub != nil {
		t.Fatalf("GetLatestSubscriptionForLead before any purchase = (%v, %v), want (nil, nil)", sub, err)
	}

	paidAt := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	var subID uuid.UUID
	if err := env.pool.QueryRow(ctx, `
		INSERT INTO user_subscriptions (studio_id, lead_id, plan_id, plan_name, amount_paid, currency, payment_status, subscription_status, created_at)
		VALUES ($1, $2, $3, 'Regression Test Plan', 4500, 'SGD', 'paid', 'active', $4)
		RETURNING id
	`, env.studioID, leadID, planID, paidAt).Scan(&subID); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.pool.Exec(context.Background(), `DELETE FROM user_subscriptions WHERE id = $1`, subID)
	})

	sub, err := repo.GetLatestSubscriptionForLead(ctx, leadID)
	if err != nil {
		t.Fatalf("GetLatestSubscriptionForLead: %v", err)
	}
	if sub == nil {
		t.Fatal("GetLatestSubscriptionForLead returned nil after a subscription was seeded")
	}
	if sub.PlanName != "Regression Test Plan" || sub.AmountPaid != 4500 || sub.Currency != "SGD" || !sub.PaidAt.Equal(paidAt) {
		t.Errorf("GetLatestSubscriptionForLead = %+v, want plan=Regression Test Plan amount=4500 SGD paidAt=%v", sub, paidAt)
	}

	body := alreadyPaidBody(ctx, repo, leadID, "Alex", "trial")
	if !strings.Contains(body, "Regression Test Plan") {
		t.Errorf("alreadyPaidBody should name the actual plan, got: %q", body)
	}
	if !strings.Contains(body, "SGD 45.00") {
		t.Errorf("alreadyPaidBody should show the amount actually paid, got: %q", body)
	}
	if !strings.Contains(body, "15 Mar 2026") {
		t.Errorf("alreadyPaidBody should show when it was paid, got: %q", body)
	}

	// End-to-end: SendTrialPaymentLink itself should surface these same
	// details once trial_purchased is set, not just a generic "already
	// paid" line.
	if _, err := env.pool.Exec(ctx, `UPDATE leads SET trial_purchased = true WHERE id = $1`, leadID); err != nil {
		t.Fatalf("mark trial_purchased: %v", err)
	}
	sentBody, err := env.msgSvc.SendTrialPaymentLink(ctx, env.studioID, conv.ID, &leadID, "Alex")
	if err != nil {
		t.Fatalf("SendTrialPaymentLink: %v", err)
	}
	if !strings.Contains(sentBody, "Regression Test Plan") || !strings.Contains(sentBody, "SGD 45.00") {
		t.Errorf("SendTrialPaymentLink for an already-purchased lead = %q, want it to include the actual plan and amount paid", sentBody)
	}
}
