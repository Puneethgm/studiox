package studios

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MemberSubscription is a lead's membership-plan subscription — the real
// cadence/renewal/lifecycle record written by the Stripe webhook
// (webhook_stripe.go), as opposed to the display-only fields on Lead
// (member_sold, status, monthly_fee, fitness_plan). This is the source of
// truth studio owners check to see who paid what, on what cadence, when the
// next charge lands, and whether a renewal is past_due/canceled.
type MemberSubscription struct {
	ID                   uuid.UUID  `json:"id"`
	LeadID               uuid.UUID  `json:"leadId"`
	LeadName             string     `json:"leadName"`
	LeadPhone            string     `json:"leadPhone"`
	PlanName             string     `json:"planName"`
	AmountPaid           int        `json:"amountPaid"`
	Currency             string     `json:"currency"`
	PaymentStatus        string     `json:"paymentStatus"`
	SubscriptionStatus   string     `json:"subscriptionStatus"`
	BillingInterval      string     `json:"billingInterval"`
	BillingIntervalCount int        `json:"billingIntervalCount"`
	StartDate            time.Time  `json:"startDate"`
	NextRenewalAt        *time.Time `json:"nextRenewalAt"`
	CanceledAt           *time.Time `json:"canceledAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	StripeSubscriptionID string     `json:"stripeSubscriptionId"`
	StripeCustomerID     string     `json:"stripeCustomerId"`
	ReceiptURL           string     `json:"receiptUrl"`
}

// ListMemberSubscriptions returns every membership subscription (active,
// past_due, canceled, superseded, completed) for the studio's leads, newest
// first. Includes superseded/canceled rows deliberately — a studio owner
// tracking "who didn't pay" needs the full picture, not just current members.
func (r *Repo) ListMemberSubscriptions(ctx context.Context, studioID uuid.UUID) ([]MemberSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT us.id, us.lead_id, l.name, l.phone, us.plan_name, us.amount_paid, us.currency,
		       us.payment_status, us.subscription_status, us.billing_interval, us.billing_interval_count,
		       us.start_date, us.next_renewal_at, us.canceled_at, us.created_at,
		       us.stripe_subscription_id, us.stripe_customer_id, us.receipt_url
		FROM user_subscriptions us
		JOIN leads l ON l.id = us.lead_id
		WHERE us.studio_id = $1
		ORDER BY us.created_at DESC
		LIMIT 500
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list member subscriptions query: %w", err)
	}
	defer rows.Close()

	out := []MemberSubscription{} // not nil — see ListMemberSubscriptionsForLead's comment
	for rows.Next() {
		var s MemberSubscription
		if err := rows.Scan(
			&s.ID, &s.LeadID, &s.LeadName, &s.LeadPhone, &s.PlanName, &s.AmountPaid, &s.Currency,
			&s.PaymentStatus, &s.SubscriptionStatus, &s.BillingInterval, &s.BillingIntervalCount,
			&s.StartDate, &s.NextRenewalAt, &s.CanceledAt, &s.CreatedAt,
			&s.StripeSubscriptionID, &s.StripeCustomerID, &s.ReceiptURL,
		); err != nil {
			return nil, fmt.Errorf("list member subscriptions scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListMemberSubscriptionsForLead returns every membership subscription for
// one lead (a trial then an upgrade, a renewal, a lapsed-then-resubscribed
// history, etc.), newest first — the "Payment" section on a lead's detail
// page. Scoped to studioID so a lead can't be looked up cross-studio.
func (r *Repo) ListMemberSubscriptionsForLead(ctx context.Context, studioID, leadID uuid.UUID) ([]MemberSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT us.id, us.lead_id, l.name, l.phone, us.plan_name, us.amount_paid, us.currency,
		       us.payment_status, us.subscription_status, us.billing_interval, us.billing_interval_count,
		       us.start_date, us.next_renewal_at, us.canceled_at, us.created_at,
		       us.stripe_subscription_id, us.stripe_customer_id, us.receipt_url
		FROM user_subscriptions us
		JOIN leads l ON l.id = us.lead_id
		WHERE us.studio_id = $1 AND us.lead_id = $2
		ORDER BY us.created_at DESC
	`, studioID, leadID)
	if err != nil {
		return nil, fmt.Errorf("list member subscriptions for lead query: %w", err)
	}
	defer rows.Close()

	// []MemberSubscription{}, not var out []MemberSubscription — a nil
	// slice marshals to JSON `null`, and the lead detail page's `.length`
	// check on the response crashed exactly on that for any lead with zero
	// subscriptions.
	out := []MemberSubscription{}
	for rows.Next() {
		var s MemberSubscription
		if err := rows.Scan(
			&s.ID, &s.LeadID, &s.LeadName, &s.LeadPhone, &s.PlanName, &s.AmountPaid, &s.Currency,
			&s.PaymentStatus, &s.SubscriptionStatus, &s.BillingInterval, &s.BillingIntervalCount,
			&s.StartDate, &s.NextRenewalAt, &s.CanceledAt, &s.CreatedAt,
			&s.StripeSubscriptionID, &s.StripeCustomerID, &s.ReceiptURL,
		); err != nil {
			return nil, fmt.Errorf("list member subscriptions for lead scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
