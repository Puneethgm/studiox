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
}

// ListMemberSubscriptions returns every membership subscription (active,
// past_due, canceled, superseded, completed) for the studio's leads, newest
// first. Includes superseded/canceled rows deliberately — a studio owner
// tracking "who didn't pay" needs the full picture, not just current members.
func (r *Repo) ListMemberSubscriptions(ctx context.Context, studioID uuid.UUID) ([]MemberSubscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT us.id, us.lead_id, l.name, l.phone, us.plan_name, us.amount_paid, us.currency,
		       us.payment_status, us.subscription_status, us.billing_interval, us.billing_interval_count,
		       us.start_date, us.next_renewal_at, us.canceled_at, us.created_at
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

	var out []MemberSubscription
	for rows.Next() {
		var s MemberSubscription
		if err := rows.Scan(
			&s.ID, &s.LeadID, &s.LeadName, &s.LeadPhone, &s.PlanName, &s.AmountPaid, &s.Currency,
			&s.PaymentStatus, &s.SubscriptionStatus, &s.BillingInterval, &s.BillingIntervalCount,
			&s.StartDate, &s.NextRenewalAt, &s.CanceledAt, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list member subscriptions scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
