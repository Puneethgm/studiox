package studios

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
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
	r.Post("/studios/{id}/payments/plan", h.updatePlan)
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
	SocialPlannerEnabled *bool               `json:"socialPlannerEnabled"`
	KnowledgeBase        *string             `json:"knowledgeBase"`
	KnowledgeBaseFiles   *[]KnowledgeBaseFile `json:"knowledgeBaseFiles"`
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
		SocialPlannerEnabled: existing.SocialPlannerEnabled,
		KnowledgeBase:        existing.KnowledgeBase,
		KnowledgeBaseFiles:   existing.KnowledgeBaseFiles,
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
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
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
		SocialPlannerEnabled: existing.SocialPlannerEnabled,
		KnowledgeBase:        existing.KnowledgeBase,
		KnowledgeBaseFiles:   existing.KnowledgeBaseFiles,
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
	if req.SocialPlannerEnabled != nil {
		input.SocialPlannerEnabled = *req.SocialPlannerEnabled
	}
	if req.KnowledgeBase != nil {
		input.KnowledgeBase = *req.KnowledgeBase
	}
	if req.KnowledgeBaseFiles != nil {
		input.KnowledgeBaseFiles = *req.KnowledgeBaseFiles
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

	httpx.JSON(w, http.StatusOK, map[string]any{
		"stripeAccountId":      s.StripeAccountID,
		"stripePublishableKey": s.StripePublishableKey,
		"stripeSecretKey":      s.StripeSecretKey,
		"subscriptionTier":     s.SubscriptionTier,
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

func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden", "cannot change plan on global scope")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	var req struct {
		SubscriptionTier string `json:"subscriptionTier"`
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

	err = h.svc.UpdatePayments(r.Context(), id, s.StripeAccountID, s.StripeSecretKey, s.StripePublishableKey, req.SubscriptionTier)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
