package studios

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
)

func (h *Handler) UpgradeStudioPlan(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "id")
	if studioID == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_studio", "Cannot upgrade global studio")
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
	var req struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	// Fetch dynamic platform plans to get the price
	val, err := h.svc.GetPlatformSetting(r.Context(), "platform_plans")
	if err != nil || val == "" {
		val = `[{"name":"Trial Pass","price":300,"cycle":"One-time"},{"name":"Growth Tier","price":999,"cycle":"Monthly"},{"name":"Pro Tier","price":1299,"cycle":"Monthly"},{"name":"Enterprise Tier","price":1599,"cycle":"Monthly"}]`
	}
	var plans []map[string]any
	json.Unmarshal([]byte(val), &plans)

	var amount int64 = -1
	for _, p := range plans {
		if p["name"] == req.Tier {
			amount = int64(p["price"].(float64)) * 100 // Convert to cents
			break
		}
	}

	if amount == -1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tier", "Invalid subscription tier")
		return
	}

	secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if secretKey == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "platform_not_configured", "Platform Stripe account is not configured")
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Create a Stripe checkout session for the existing studio
	params := &stripe.CheckoutSessionParams{
		CustomerEmail:      stripe.String(studio.ContactEmail),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("sgd"),
					UnitAmount: stripe.Int64(amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("1herosocial.ai - %s Upgrade", req.Tier)),
					},
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String("month"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("subscription"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/admin/studios/%s/settings?upgrade=success", frontendURL, studioID)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/admin/studios/%s/settings", frontendURL, studioID)),
		Metadata: map[string]string{
			"plan_tier": req.Tier,
			"studio_id": studioID,
			"is_upgrade": "true",
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"studio_id": studioID,
			},
		},
	}

	if req.Tier == "Trial Pass" {
		params.Mode = stripe.String("payment")
		params.LineItems[0].PriceData.Recurring = nil
	}

	session, err := sc.CheckoutSessions.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("Failed to create checkout session: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"url": session.URL,
	})
}
