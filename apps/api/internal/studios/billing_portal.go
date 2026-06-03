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

func (h *Handler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "id")
	if studioID == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_studio", "Cannot open portal for global studio")
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
		httpx.WriteError(w, http.StatusInternalServerError, "platform_not_configured", "Platform Stripe account is not configured")
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	// Find customer by email
	params := &stripe.CustomerListParams{
		Email: stripe.String(studio.ContactEmail),
	}
	iter := sc.Customers.List(params)
	var customerID string
	if iter.Next() {
		customerID = iter.Customer().ID
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	if customerID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "no_subscription", "You do not have an active subscription to manage. Please select a plan to upgrade.")
		return
	}

	portalParams := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(fmt.Sprintf("%s/admin/studios/%s/settings", frontendURL, studio.ID)),
	}

	session, err := sc.BillingPortalSessions.New(portalParams)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", "Failed to create portal session")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"url": session.URL,
	})
}
