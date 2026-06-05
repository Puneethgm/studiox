package studios

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
)

// SyncBillingStatus checks Stripe directly for the studio's subscription status
// and updates the DB accordingly. This is the fallback for when webhooks don't fire (local dev).
func (h *Handler) SyncBillingStatus(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "id")
	if studioID == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_studio", "Cannot sync global studio")
		return
	}

	parsedID, err := uuid.Parse(studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid studio ID")
		return
	}
	studio, err := h.svc.repo.GetByID(r.Context(), parsedID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Studio not found")
		return
	}

	secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if secretKey == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"synced": false, "reason": "no_stripe_key"})
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	// Find customer by email
	custParams := &stripe.CustomerListParams{
		Email: stripe.String(studio.ContactEmail),
	}
	iter := sc.Customers.List(custParams)
	if !iter.Next() {
		// No customer at all — they've never paid, mark as canceled if they have a non-free tier
		if os.Getenv("API_ENV") != "local" && studio.SubscriptionTier != "" && studio.SubscriptionTier != "Trial Pass" {
			_ = h.svc.UpdatePayments(r.Context(), parsedID, "", "", "", "", "canceled")
			fmt.Printf("[BillingSync] No Stripe customer found for studio %s, marking canceled\n", studioID)
		}
		tier := "canceled"
		if os.Getenv("API_ENV") == "local" {
			tier = studio.SubscriptionTier
			if tier == "" {
				tier = "Trial Pass"
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"synced": true, "tier": tier})
		return
	}
	customerID := iter.Customer().ID

	// List all subscriptions for this customer
	subParams := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
	}
	subIter := sc.Subscriptions.List(subParams)

	var activeSub *stripe.Subscription
	for subIter.Next() {
		sub := subIter.Subscription()
		if sub.Status == "active" || sub.Status == "trialing" {
			activeSub = sub
			break
		}
	}

	currentTier := studio.SubscriptionTier

	if activeSub == nil {
		// No active subscription → mark canceled
		if os.Getenv("API_ENV") != "local" && currentTier != "canceled" && currentTier != "" {
			_ = h.svc.UpdatePayments(r.Context(), parsedID, "", "", "", "", "canceled")
			fmt.Printf("[BillingSync] No active sub found for studio %s, marking canceled\n", studioID)
		}
		tier := "canceled"
		if os.Getenv("API_ENV") == "local" {
			tier = currentTier
			if tier == "" {
				tier = "Trial Pass"
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"synced": true, "tier": tier})
		return
	}

	// Active sub found — check plan_tier from subscription metadata first
	planTier := activeSub.Metadata["plan_tier"]

	// If not on subscription, check recent checkout sessions for the studio
	if planTier == "" {
		csParams := &stripe.CheckoutSessionListParams{
			Customer: stripe.String(customerID),
		}
		csIter := sc.CheckoutSessions.List(csParams)
		for csIter.Next() {
			cs := csIter.CheckoutSession()
			if cs.Metadata["studio_id"] == studioID && cs.Metadata["plan_tier"] != "" {
				planTier = cs.Metadata["plan_tier"]
				break
			}
		}
	}

	// If we have a tier and it differs from what's stored (or stored is canceled), update
	if planTier != "" && (planTier != currentTier || currentTier == "canceled" || currentTier == "past_due") {
		_ = h.svc.UpdatePayments(r.Context(), parsedID, "", "", "", "", planTier)
		fmt.Printf("[BillingSync] Synced studio %s tier from '%s' to '%s'\n", studioID, currentTier, planTier)
		httpx.JSON(w, http.StatusOK, map[string]any{"synced": true, "tier": planTier})
		return
	}

	// Active sub exists but stored tier is canceled/past_due — restore to active (use stored or fallback)
	if currentTier == "canceled" || currentTier == "past_due" {
		restoreTier := planTier
		if restoreTier == "" {
			restoreTier = "Growth Tier" // fallback if no metadata found
		}
		_ = h.svc.UpdatePayments(r.Context(), parsedID, "", "", "", "", restoreTier)
		fmt.Printf("[BillingSync] Restored studio %s to active tier '%s'\n", studioID, restoreTier)
		httpx.JSON(w, http.StatusOK, map[string]any{"synced": true, "tier": restoreTier})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"synced": false, "tier": currentTier})
}
