package studios

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
	
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
	stripeoauth "github.com/stripe/stripe-go/v78/oauth"
)

type Handler struct {
	svc             *Service
	credentialsPath string
}

func NewHandler(svc *Service, credentialsPath string) *Handler {
	return &Handler{svc: svc, credentialsPath: credentialsPath}
}

// AdminRoutes are super-admin only — only the platform owner manages studios.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Use(identity.RequireRole(identity.RoleSuperAdmin))
	r.Get("/studios", h.list)
	r.Post("/studios", h.create)
	r.Get("/studios/{id}", h.get)
	r.Patch("/studios/{id}", h.update)

	r.Get("/google-credentials", h.getGoogleCredentials)
	r.Post("/google-credentials", h.uploadGoogleCredentials)
}

// SelfRoutes are for any authenticated user. Studio admins use this to fetch
// their own studio for the settings page.
func (h *Handler) SelfRoutes(r chi.Router) {
	r.Get("/studios/{id}", h.getScoped)
	r.Patch("/studios/{id}", h.updateScoped)
	r.Get("/studios/{id}/payments", h.getPayments)
	r.Post("/studios/{id}/payments/stripe", h.linkStripe)
	r.Get("/studios/{id}/billing/history", h.getBillingHistory)
	r.Post("/studios/{id}/trial-checkout", h.createTrialCheckout)
}

// PublicRoutes expose the studio's brand info for the public form to render.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/public/studios/{slug}", h.publicGet)
}

// RequireActiveStudio blocks studio_admins whose studio has been marked
// inactive by a super admin. Super admins always pass through (so they can
// access an inactive studio in order to reactivate it). Returns a structured
// `studio_inactive` error so the frontend can render a clean lockout screen
// rather than a generic 403.
func (h *Handler) RequireActiveStudio(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := identity.MustClaims(r.Context())
		if c.IsSuper() {
			next.ServeHTTP(w, r)
			return
		}
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}

		// Prevent IDOR: Ensure the user is only accessing their own studio
		requestedStudioID := chi.URLParam(r, "studioId")
		if requestedStudioID != "" && requestedStudioID != "global" && requestedStudioID != c.StudioID.String() {
			// Also check "id" just in case the param is named "id" in some routes
			if chi.URLParam(r, "id") == "" || chi.URLParam(r, "id") != c.StudioID.String() {
				httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to access this studio")
				return
			}
		}

		s, err := h.svc.GetByID(r.Context(), *c.StudioID)
		if err != nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "studio not accessible")
			return
		}
		if !s.Active {
			httpx.WriteError(w, http.StatusForbidden, "studio_inactive",
				"this studio has been deactivated by the platform admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ----- super-admin handlers -----

type createReq struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	BrandColor    string `json:"brandColor"`
	LogoURL       string `json:"logoUrl"`
	ContactEmail  string `json:"contactEmail"`
	AdminEmail           string `json:"adminEmail"`
	AdminPassword        string `json:"adminPassword"`
	SocialPlannerEnabled bool   `json:"socialPlannerEnabled"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.BrandColor == "" {
		req.BrandColor = "#7c3aed"
	}
	res, errs, err := h.svc.CreateStudioWithAdmin(r.Context(), CreateStudioInput{
		Slug:          req.Slug,
		Name:          req.Name,
		BrandColor:    req.BrandColor,
		LogoURL:       req.LogoURL,
		ContactEmail:         req.ContactEmail,
		AdminEmail:           req.AdminEmail,
		AdminPassword:        req.AdminPassword,
		SocialPlannerEnabled: req.SocialPlannerEnabled,
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"studios": list})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

type updateReq struct {
	Name                 *string             `json:"name"`
	BrandColor           *string             `json:"brandColor"`
	LogoURL              *string             `json:"logoUrl"`
	ContactEmail         *string             `json:"contactEmail"`
	Active               *bool               `json:"active"`
	AvailabilitySlots    *[]AvailabilitySlot `json:"availabilitySlots"`
	AvailabilityTimezone *string             `json:"availabilityTimezone"`
	GeminiAPIKey         *string             `json:"geminiApiKey"`
	MetaAppID            *string             `json:"metaAppId"`
	MetaAppSecret        *string             `json:"metaAppSecret"`
	GoogleClientID       *string             `json:"googleClientId"`
	GoogleClientSecret   *string             `json:"googleClientSecret"`
	GoogleDeveloperToken *string             `json:"googleDeveloperToken"`
	SocialPlannerEnabled *bool               `json:"socialPlannerEnabled"`
	KnowledgeBase        *string             `json:"knowledgeBase"`
	KnowledgeBaseFiles   *[]KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	TrialAmountSGD       *int                `json:"trialAmountSgd"`
	TrialAmountINR       *int                `json:"trialAmountInr"`
	TrialAmountUSD       *int                `json:"trialAmountUsd"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req updateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	existing, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	input := UpdateStudioInput{
		Name:                 existing.Name,
		BrandColor:           existing.BrandColor,
		LogoURL:              existing.LogoURL,
		ContactEmail:         existing.ContactEmail,
		Active:               existing.Active,
		AvailabilitySlots:    existing.AvailabilitySlots,
		AvailabilityTimezone: existing.AvailabilityTimezone,
		GeminiAPIKey:         existing.GeminiAPIKey,
		MetaAppID:            existing.MetaAppID,
		MetaAppSecret:        existing.MetaAppSecret,
		GoogleClientID:       existing.GoogleClientID,
		GoogleClientSecret:   existing.GoogleClientSecret,
		GoogleDeveloperToken: existing.GoogleDeveloperToken,
		SocialPlannerEnabled: existing.SocialPlannerEnabled,
		KnowledgeBase:        existing.KnowledgeBase,
		KnowledgeBaseFiles:   existing.KnowledgeBaseFiles,
		TrialAmountSGD:       existing.TrialAmountSGD,
		TrialAmountINR:       existing.TrialAmountINR,
		TrialAmountUSD:       existing.TrialAmountUSD,
	}
	if req.Name != nil {
		input.Name = *req.Name
	}
	if req.BrandColor != nil {
		input.BrandColor = *req.BrandColor
	}
	if req.LogoURL != nil {
		input.LogoURL = *req.LogoURL
	}
	if req.ContactEmail != nil {
		input.ContactEmail = *req.ContactEmail
	}
	if req.Active != nil {
		input.Active = *req.Active
	}
	if req.AvailabilitySlots != nil {
		input.AvailabilitySlots = *req.AvailabilitySlots
	}
	if req.AvailabilityTimezone != nil {
		input.AvailabilityTimezone = *req.AvailabilityTimezone
	}
	if req.GeminiAPIKey != nil {
		input.GeminiAPIKey = *req.GeminiAPIKey
	}
	if req.MetaAppID != nil {
		input.MetaAppID = *req.MetaAppID
	}
	if req.MetaAppSecret != nil {
		input.MetaAppSecret = *req.MetaAppSecret
	}
	if req.GoogleClientID != nil {
		input.GoogleClientID = *req.GoogleClientID
	}
	if req.GoogleClientSecret != nil {
		input.GoogleClientSecret = *req.GoogleClientSecret
	}
	if req.GoogleDeveloperToken != nil {
		input.GoogleDeveloperToken = *req.GoogleDeveloperToken
	}
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
	}
	if req.TrialAmountSGD != nil {
		input.TrialAmountSGD = *req.TrialAmountSGD
	}
	if req.TrialAmountINR != nil {
		input.TrialAmountINR = *req.TrialAmountINR
	}
	if req.TrialAmountUSD != nil {
		input.TrialAmountUSD = *req.TrialAmountUSD
	}

	errs, err := h.svc.Update(r.Context(), id, input)
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	updated, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

// ----- studio-admin scoped handlers -----
//
// A studio_admin can read/update only their own studio. Super_admins can use
// the AdminRoutes endpoints above for any studio. We fail closed: if the path
// id doesn't match the caller's claim, return 403.

func (h *Handler) getScoped(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	// Super admins use the URL param; studio_admins always get their own studio.
	var studioID uuid.UUID
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
			return
		}
		studioID = id
	} else {
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}
		studioID = *c.StudioID
	}
	s, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *Handler) updateScoped(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	// Super admins use the URL param; studio_admins always update their own studio.
	var studioID uuid.UUID
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
			return
		}
		studioID = id
	} else {
		if c.StudioID == nil {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
			return
		}
		studioID = *c.StudioID
	}
	var req updateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	existing, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	input := UpdateStudioInput{
		Name:                 existing.Name,
		BrandColor:           existing.BrandColor,
		LogoURL:              existing.LogoURL,
		ContactEmail:         existing.ContactEmail,
		Active:               existing.Active,
		AvailabilitySlots:    existing.AvailabilitySlots,
		AvailabilityTimezone: existing.AvailabilityTimezone,
		GeminiAPIKey:         existing.GeminiAPIKey,
		MetaAppID:            existing.MetaAppID,
		MetaAppSecret:        existing.MetaAppSecret,
		GoogleClientID:       existing.GoogleClientID,
		GoogleClientSecret:   existing.GoogleClientSecret,
		GoogleDeveloperToken: existing.GoogleDeveloperToken,
		SocialPlannerEnabled: existing.SocialPlannerEnabled,
		KnowledgeBase:        existing.KnowledgeBase,
		KnowledgeBaseFiles:   existing.KnowledgeBaseFiles,
		TrialAmountSGD:       existing.TrialAmountSGD,
		TrialAmountINR:       existing.TrialAmountINR,
		TrialAmountUSD:       existing.TrialAmountUSD,
	}
	if req.Name != nil {
		input.Name = *req.Name
	}
	if req.BrandColor != nil {
		input.BrandColor = *req.BrandColor
	}
	if req.LogoURL != nil {
		input.LogoURL = *req.LogoURL
	}
	if req.ContactEmail != nil {
		input.ContactEmail = *req.ContactEmail
	}
	if req.Active != nil {
		input.Active = *req.Active
	}
	if req.AvailabilitySlots != nil {
		input.AvailabilitySlots = *req.AvailabilitySlots
	}
	if req.AvailabilityTimezone != nil {
		input.AvailabilityTimezone = *req.AvailabilityTimezone
	}
	if req.GeminiAPIKey != nil {
		input.GeminiAPIKey = *req.GeminiAPIKey
	}
	if req.MetaAppID != nil {
		input.MetaAppID = *req.MetaAppID
	}
	if req.MetaAppSecret != nil {
		input.MetaAppSecret = *req.MetaAppSecret
	}
	if req.GoogleClientID != nil {
		input.GoogleClientID = *req.GoogleClientID
	}
	if req.GoogleClientSecret != nil {
		input.GoogleClientSecret = *req.GoogleClientSecret
	}
	if req.GoogleDeveloperToken != nil {
		input.GoogleDeveloperToken = *req.GoogleDeveloperToken
	}
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
	}
	if req.TrialAmountSGD != nil {
		input.TrialAmountSGD = *req.TrialAmountSGD
	}
	if req.TrialAmountINR != nil {
		input.TrialAmountINR = *req.TrialAmountINR
	}
	if req.TrialAmountUSD != nil {
		input.TrialAmountUSD = *req.TrialAmountUSD
	}

	errs, err := h.svc.Update(r.Context(), studioID, input)
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	updated, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

// ----- public -----

type publicRes struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	BrandColor string `json:"brandColor"`
	LogoURL    string `json:"logoUrl"`
}

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	s, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil || !s.Active {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}
	httpx.JSON(w, http.StatusOK, publicRes{
		Slug:       s.Slug,
		Name:       s.Name,
		BrandColor: s.BrandColor,
		LogoURL:    s.LogoURL,
	})
}

func (h *Handler) getGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	if h.credentialsPath == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}

	data, err := os.ReadFile(h.credentialsPath)
	if err != nil {
		if os.IsNotExist(err) {
			httpx.JSON(w, http.StatusOK, map[string]any{"configured": false})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	var creds struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"configured": false, "error": "invalid json format"})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"configured":  creds.Type == "service_account" && creds.ClientEmail != "",
		"clientEmail": creds.ClientEmail,
		"projectId":   creds.ProjectID,
	})
}

func (h *Handler) uploadGoogleCredentials(w http.ResponseWriter, r *http.Request) {
	if h.credentialsPath == "" {
		httpx.WriteError(w, http.StatusBadRequest, "disabled", "Google Sheets credentials path not configured in env")
		return
	}

	// 1MB max for service account JSON
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	var creds struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(bytes, &creds); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "file is not valid JSON")
		return
	}

	if creds.Type != "service_account" || creds.ClientEmail == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_credentials", "file is not a valid Google service account JSON key file")
		return
	}

	// Ensure the parent directory of h.credentialsPath exists
	dir := filepath.Dir(h.credentialsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create secrets directory")
		return
	}

	// Write file
	if err := os.WriteFile(h.credentialsPath, bytes, 0600); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to save file: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"configured":  true,
		"clientEmail": creds.ClientEmail,
		"projectId":   creds.ProjectID,
	})
}

func (h *Handler) getPayments(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	// If id is 'global', return empty config
	if idStr == "global" {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"stripeAccountId":      "",
			"stripePublishableKey": "",
			"stripeSecretKey":      "",
			"subscriptionTier":     "pro",
		})
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	hasSecretKey := s.StripeSecretKey != ""
	
	httpx.JSON(w, http.StatusOK, map[string]any{
		"stripeAccountId":      s.StripeAccountID,
		"stripePublishableKey": s.StripePublishableKey,
		"hasStripeSecretKey":   hasSecretKey,
		"subscriptionTier":     s.SubscriptionTier,
		"trialAmountSgd":       s.TrialAmountSGD,
		"trialAmountInr":       s.TrialAmountINR,
		"trialAmountUsd":       s.TrialAmountUSD,
	})
}

func (h *Handler) createTrialCheckout(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden", "cannot create checkout on global scope")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	var req struct {
		Currency    string `json:"currency"`    // "sgd", "inr", "usd"
		CustomerPhone string `json:"customerPhone"` // E.164 format e.g. 6591234567
		CustomerName  string `json:"customerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	if s.StripeSecretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "stripe_not_configured", "Stripe is not connected for this studio")
		return
	}

	// Determine amount and currency
	var amount int64
	cur := strings.ToLower(req.Currency)
	switch cur {
	case "inr":
		amount = int64(s.TrialAmountINR)
	case "usd":
		amount = int64(s.TrialAmountUSD)
	default:
		cur = "sgd"
		amount = int64(s.TrialAmountSGD)
	}
	if amount == 0 {
		amount = 2500 // default 25.00 SGD in cents
	}

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Use Checkout Session with inline price_data (no pre-created product needed)
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(cur),
					UnitAmount: stripe.Int64(amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(fmt.Sprintf("%s Trial Session", s.Name)),
						Description: stripe.String("Secure your trial workout session at " + s.Name),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("payment"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/payment-success?studio=%s&session_id={CHECKOUT_SESSION_ID}", frontendURL, s.Slug)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/payment-cancelled?studio=%s", frontendURL, s.Slug)),
	}
	if req.CustomerPhone != "" {
		params.Metadata = map[string]string{
			"customer_phone": req.CustomerPhone,
			"customer_name":  req.CustomerName,
			"studio_id":      id.String(),
		}
	}

	session, err := sc.CheckoutSessions.New(params)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", fmt.Sprintf("Failed to create checkout session: %v", err))
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"url":      session.URL,
		"amount":   amount,
		"currency": cur,
	})
}

func (h *Handler) linkStripe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden", "cannot link stripe on global scope")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	var req struct {
		StripeAccountId      string `json:"stripeAccountId"`
		StripePublishableKey string `json:"stripePublishableKey"`
		StripeSecretKey      string `json:"stripeSecretKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	err = h.svc.UpdatePayments(r.Context(), id, req.StripeAccountId, req.StripeSecretKey, req.StripePublishableKey, s.SubscriptionTier)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
func (h *Handler) getBillingHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		httpx.JSON(w, http.StatusOK, map[string]any{"invoices": []any{}, "stats": map[string]any{}})
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	s, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	if s.StripeSecretKey == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"invoices": []any{}, "stats": map[string]any{}})
		return
	}

	sc := &client.API{}
	sc.Init(s.StripeSecretKey, nil)

	// Checkout Sessions create PaymentIntents (not Invoices).
	// Query PaymentIntents to show all trial booking payments.
	piParams := &stripe.PaymentIntentListParams{}
	piParams.Limit = stripe.Int64(20)
	piIter := sc.PaymentIntents.List(piParams)

	var invoices []map[string]any
	var lifetimePaid int64
	var lifetimePaidByCurrency = map[string]int64{}

	for piIter.Next() {
		pi := piIter.PaymentIntent()
		if pi.Status != stripe.PaymentIntentStatusSucceeded {
			continue
		}

		receiptURL := ""
		description := pi.Description
		if description == "" {
			description = "Trial Session Payment"
		}
		// Try to get receipt from latest charge
		if pi.LatestCharge != nil {
			receiptURL = pi.LatestCharge.ReceiptURL
		}

		cur := string(pi.Currency)
		invoices = append(invoices, map[string]any{
			"id":          pi.ID,
			"number":      pi.ID[3:11], // short reference
			"amount_due":  pi.Amount,
			"amount_paid": pi.AmountReceived,
			"currency":    cur,
			"status":      "paid",
			"created":     pi.Created,
			"hosted_invoice_url": receiptURL,
			"invoice_pdf": receiptURL,
			"description": description,
			"metadata":    pi.Metadata,
		})

		lifetimePaidByCurrency[cur] += pi.AmountReceived
		lifetimePaid += pi.AmountReceived
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"invoices": invoices,
		"stats": map[string]any{
			"outstandingSGD":  int64(0),
			"outstandingINR":  int64(0),
			"lifetimePaidSGD": lifetimePaidByCurrency["sgd"],
			"lifetimePaidINR": lifetimePaidByCurrency["inr"],
			"lifetimePaidUSD": lifetimePaidByCurrency["usd"],
			"lifetimePaid":    lifetimePaid,
		},
	})
}

// ----- Stripe Connect OAuth (Phase 4) -----

func (h *Handler) StripeConnectRedirect(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "studioId")
	if studioID == "" {
		studioID = chi.URLParam(r, "id") // Fallback just in case
	}
	// The client_id should come from environment variables.
	clientID := os.Getenv("STRIPE_CLIENT_ID")
	redirectURI := fmt.Sprintf("%s/api/v1/auth/stripe/callback", os.Getenv("PUBLIC_URL"))
	
	stripeOAuthURL := fmt.Sprintf(
		"https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&redirect_uri=%s&state=%s",
		clientID, redirectURI, studioID,
	)
	
	http.Redirect(w, r, stripeOAuthURL, http.StatusTemporaryRedirect)
}

func (h *Handler) StripeConnectCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state") // Studio ID passed in state
	
	if code == "" || state == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Missing code or state")
		return
	}
	
	studioID, err := uuid.Parse(state)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "Invalid state parameter")
		return
	}
	
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Stripe secret key not configured on platform")
		return
	}

	params := &stripe.OAuthTokenParams{
		GrantType: stripe.String("authorization_code"),
		Code:      stripe.String(code),
	}

	token, err := stripeoauth.New(params)
	if err != nil {
		fmt.Printf("STRIPE OAUTH ERROR: %v\n", err)
		httpx.WriteError(w, http.StatusInternalServerError, "stripe_error", "Failed to authenticate with Stripe")
		return
	}

	// Update the studio's payment configuration with the connected account ID
	s, err := h.svc.GetByID(r.Context(), studioID)
	if err == nil {
		_ = h.svc.UpdatePayments(r.Context(), studioID, token.StripeUserID, "", "", s.SubscriptionTier)
	}

	// Redirect back to frontend
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	http.Redirect(w, r, fmt.Sprintf("%s/admin/studios/%s/settings?tab=integrations", frontendURL, studioID), http.StatusTemporaryRedirect)
}

