package studios

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/s3"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
	stripeoauth "github.com/stripe/stripe-go/v78/oauth"
)

type Handler struct {
	svc             *Service
	credentialsPath string
	s3Uploader      *s3.Uploader
}

func NewHandler(svc *Service, credentialsPath string, s3Uploader *s3.Uploader) *Handler {
	return &Handler{svc: svc, credentialsPath: credentialsPath, s3Uploader: s3Uploader}
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
	r.Post("/studios/{id}/logo", h.uploadLogo)

	// Plans routes
	r.Get("/studios/{id}/plans", h.listPlans)
	r.Put("/studios/{id}/plans/{planId}", h.updatePlan)
	r.Get("/studios/{id}/payments", h.getPayments)
	r.Post("/studios/{id}/payments/stripe", h.linkStripe)
	// Platform Plans route
	r.Put("/studios/global/plans", h.UpdatePlatformPlans)

	r.Get("/studios/{id}/billing/history", h.getBillingHistory)
	r.Post("/studios/{id}/billing/upgrade", h.UpgradeStudioPlan)
	r.Post("/studios/{id}/billing/portal", h.CreatePortalSession)
	r.Post("/studios/{id}/billing/sync", h.SyncBillingStatus)
	r.Post("/studios/{id}/trial-checkout", h.createTrialCheckout)

	// Account deletion
	r.Delete("/studios/{id}/delete-account", h.deleteAccount)
}

// PublicRoutes expose the studio's brand info for the public form to render.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/public/studios/{slug}", h.publicGet)
	r.Get("/public/platform/plans", h.GetPlatformPlans)
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

func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
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

	// 5MB max for logo image
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	// Validate file type (only images)
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_type", "only image files (JPEG, PNG, WebP, GIF) are allowed")
		return
	}

	// Create a unique filename using studio ID
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		// Infer extension from content type
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "image/gif":
			ext = ".gif"
		}
	}

	// Read file data
	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	// Upload to S3 if configured, otherwise fall back to disk
	var logoURL string
	if h.s3Uploader != nil {
		key := fmt.Sprintf("logos/%s%s", studioID.String(), ext)
		url, err := h.s3Uploader.UploadImage(r.Context(), key, bytes, contentType)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "s3_upload", "failed to upload logo")
			return
		}
		logoURL = url
	} else {
		// Fallback to disk storage if S3 not configured
		filename := fmt.Sprintf("logo_%s%s", studioID.String(), ext)
		filepath := filepath.Join("./uploads", filename)
		if err := os.MkdirAll("./uploads", 0755); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create uploads directory")
			return
		}
		if err := os.WriteFile(filepath, bytes, 0644); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to save file")
			return
		}
		logoURL = fmt.Sprintf("/uploads/%s", filename)
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"logoUrl": logoURL,
	})
}

func (h *Handler) UploadSocialPostImage(w http.ResponseWriter, r *http.Request) {
	c := identity.MustClaims(r.Context())
	if c.IsSuper() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "super admins cannot upload social post images")
		return
	}

	studioIDStr := chi.URLParam(r, "studioId")
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}

	// Verify the user owns the studio
	if c.StudioID == nil || c.StudioID.String() != studioID.String() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "cannot access this studio")
		return
	}

	// 10MB max for social media images
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	// Validate image type
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_type", "only JPEG, PNG, WebP, GIF allowed")
		return
	}

	// Read file data
	bytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to read file")
		return
	}

	// Get file extension
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = map[string]string{
			"image/jpeg": ".jpg",
			"image/png":  ".png",
			"image/webp": ".webp",
			"image/gif":  ".gif",
		}[contentType]
	}

	var mediaURL string

	// Try S3 first if configured, otherwise fall back to disk
	if h.s3Uploader != nil {
		key := fmt.Sprintf(
			"social-posts/%s/%d_%s%s",
			studioID.String(),
			time.Now().Unix(),
			uuid.New().String()[:8],
			ext,
		)
		url, err := h.s3Uploader.UploadImage(r.Context(), key, bytes, contentType)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "s3_upload", fmt.Sprintf("S3 upload failed: %v", err))
			return
		}
		mediaURL = url
	} else {
		// Fallback to disk storage if S3 not configured
		filename := fmt.Sprintf("social_%s_%d_%s%s", studioID.String(), time.Now().Unix(), uuid.New().String()[:8], ext)
		filepath := filepath.Join("./uploads", filename)
		if err := os.MkdirAll("./uploads", 0755); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to create uploads directory: %v", err))
			return
		}
		if err := os.WriteFile(filepath, bytes, 0644); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("failed to save file: %v", err))
			return
		}
		mediaURL = fmt.Sprintf("/uploads/%s", filename)
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"mediaUrl": mediaURL})
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
	// If id is 'global', return platform settings
	if idStr == "global" {
		accountId, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_account_id")
		publishableKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_publishable_key")
		secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
		webhookSecret, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_webhook_secret")
		
		httpx.JSON(w, http.StatusOK, map[string]any{
			"stripeAccountId":        accountId,
			"stripePublishableKey":   publishableKey,
			"hasStripeSecretKey":     secretKey != "",
			"hasStripeWebhookSecret": webhookSecret != "",
			"subscriptionTier":       "platform",
			"trialAmountSgd":         0,
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
	hasWebhookSecret := s.StripeWebhookSecret != ""
	
	httpx.JSON(w, http.StatusOK, map[string]any{
		"stripeAccountId":        s.StripeAccountID,
		"stripePublishableKey":   s.StripePublishableKey,
		"hasStripeSecretKey":     hasSecretKey,
		"hasStripeWebhookSecret": hasWebhookSecret,
		"subscriptionTier":       s.SubscriptionTier,
		"trialAmountSgd":         s.TrialAmountSGD,
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
	amount := int64(s.TrialAmountSGD)
	cur := "sgd"
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

func (h *Handler) CreatePlatformCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	secretKey, _ := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if secretKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "platform_not_configured", "Platform Stripe account is not configured")
		return
	}

	// Fetch dynamic platform plans
	val, err := h.svc.GetPlatformSetting(r.Context(), "platform_plans")
	if err != nil || val == "" {
		val = `[{"name":"Trial Pass","price":300,"cycle":"One-time","description":"Entry-level setup to validate AI integration and lead generation.","features":["1 Connected Channel","Basic AI Auto-Replies (200/mo)","1-day automated follow-up","Google Sheets contact sync"]},{"name":"Growth Tier","price":999,"cycle":"Monthly","description":"Automate workflows, payments, and client acquisition.","features":["3 Connected Channels","Full AI Auto-Replies (2,000/mo)","Dedicated Knowledge Base","Visual drag-and-drop Pipeline","Stripe account integration"]},{"name":"Pro Tier","price":1299,"cycle":"Monthly","description":"For active studios looking to scale reach via social media and paid advertising.","features":["8 Connected Channels","Extended AI Auto-Replies (10,000/mo)","Dual model routing (Gemini + Claude)","Advanced Social Planner","Google Ads Channel Integration","Studio Plan Option (Scheduling)"],"highlight":true},{"name":"Enterprise Tier","price":1599,"cycle":"Monthly","description":"Maximum scale, custom branding, and multi-location management.","features":["Unlimited Connected Channels","Unlimited AI Auto-Replies","Multi-Location Hub","Whitelabel Dashboard","Enterprise Studio Plan Option","Priority Support SLA"]}]`
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

	sc := &client.API{}
	sc.Init(secretKey, nil)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("sgd"),
					UnitAmount: stripe.Int64(amount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("1herosocial.ai - %s", req.Tier)),
					},
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String("month"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String("subscription"),
		SuccessURL: stripe.String(fmt.Sprintf("%s/onboarding?session_id={CHECKOUT_SESSION_ID}", frontendURL)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/pricing", frontendURL)),
		Metadata: map[string]string{
			"plan_tier": req.Tier,
		},
	}

	// For Trial Pass, it's one-time
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

func (h *Handler) ProvisionPlatformStudio(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionId     string `json:"sessionId"`
		StudioName    string `json:"studioName"`
		AdminPassword string `json:"adminPassword"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	secretKey, err := h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
	if err != nil || secretKey == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "not_configured", "Platform Stripe is not configured")
		return
	}

	sc := &client.API{}
	sc.Init(secretKey, nil)

	session, err := sc.CheckoutSessions.Get(req.SessionId, nil)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_session", "Failed to retrieve checkout session")
		return
	}

	if session.PaymentStatus != "paid" {
		httpx.WriteError(w, http.StatusBadRequest, "unpaid_session", "Checkout session is not paid")
		return
	}

	customerEmail := ""
	if session.CustomerDetails != nil && session.CustomerDetails.Email != "" {
		customerEmail = session.CustomerDetails.Email
	} else if session.CustomerEmail != "" {
		customerEmail = session.CustomerEmail
	}

	if customerEmail == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_email", "Checkout session does not have an associated email")
		return
	}

	tier := ""
	if session.Metadata != nil {
		tier = session.Metadata["plan_tier"]
	}
	if tier == "" {
		tier = "Growth Tier" // fallback
	}

	// Try to create the studio
	res, errs, err := h.svc.CreateStudioWithAdmin(r.Context(), CreateStudioInput{
		Slug:                 "", // will auto-generate from Name
		Name:                 req.StudioName,
		BrandColor:           "#7c3aed",
		LogoURL:              "",
		ContactEmail:         customerEmail,
		AdminEmail:           customerEmail,
		AdminPassword:        req.AdminPassword,
		SocialPlannerEnabled: true,
	})

	if errs != nil {
		// If the admin email is already in use, or slug taken
		if _, ok := errs["adminEmail"]; ok {
			// This email is already registered. They should probably just link it, but for now we error.
			httpx.WriteError(w, http.StatusBadRequest, "email_taken", "An account with this email already exists.")
			return
		}
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	// Assign the subscription tier based on the payment
	if res != nil && res.Studio != nil {
		// Just update the tier via direct DB update or service wrapper
		_ = h.svc.UpdatePayments(r.Context(), res.Studio.ID, "", "", "", "", tier)
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "studioId": res.Studio.ID})
}

func (h *Handler) linkStripe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	var req struct {
		StripeAccountId      string `json:"stripeAccountId"`
		StripePublishableKey string `json:"stripePublishableKey"`
		StripeSecretKey      string `json:"stripeSecretKey"`
		StripeWebhookSecret  string `json:"stripeWebhookSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	if idStr == "global" {
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_account_id", req.StripeAccountId); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_account_id")
			return
		}
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_publishable_key", req.StripePublishableKey); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_publishable_key")
			return
		}
		if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_secret_key", req.StripeSecretKey); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_secret_key")
			return
		}
		if req.StripeWebhookSecret != "" {
			if err := h.svc.UpdatePlatformSetting(r.Context(), "stripe_webhook_secret", req.StripeWebhookSecret); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update stripe_webhook_secret")
				return
			}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
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

	err = h.svc.UpdatePayments(r.Context(), id, req.StripeAccountId, req.StripeSecretKey, req.StripePublishableKey, req.StripeWebhookSecret, s.SubscriptionTier)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
func (h *Handler) getBillingHistory(w http.ResponseWriter, r *http.Request) {
	var stripeSecretKey string

	idStr := chi.URLParam(r, "id")
	if idStr == "global" {
		var err error
		stripeSecretKey, err = h.svc.GetPlatformSetting(r.Context(), "stripe_secret_key")
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if stripeSecretKey == "" {
			httpx.WriteError(w, http.StatusInternalServerError, "wtf", "key is empty for global")
			return
		}
	} else {
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
		stripeSecretKey = s.StripeSecretKey
	}

	if stripeSecretKey == "" {
		httpx.JSON(w, http.StatusOK, map[string]any{"invoices": []any{}, "stats": map[string]any{}})
		return
	}

	sc := &client.API{}
	sc.Init(stripeSecretKey, nil)

	// Checkout Sessions create PaymentIntents (not Invoices).
	// Query PaymentIntents to show all trial booking payments.
	piParams := &stripe.PaymentIntentListParams{}
	
	// Read optional date filters
	startDateStr := r.URL.Query().Get("startDate")
	endDateStr := r.URL.Query().Get("endDate")
	
	if startDateStr != "" || endDateStr != "" {
		createdParams := &stripe.RangeQueryParams{}
		if startDateStr != "" {
			if startUnix, err := strconv.ParseInt(startDateStr, 10, 64); err == nil {
				createdParams.GreaterThanOrEqual = startUnix
			}
		}
		if endDateStr != "" {
			if endUnix, err := strconv.ParseInt(endDateStr, 10, 64); err == nil {
				createdParams.LesserThanOrEqual = endUnix
			}
		}
		piParams.CreatedRange = createdParams
		piParams.Limit = stripe.Int64(100) // Increase limit when filtering
	} else {
		piParams.Limit = stripe.Int64(20) // Default limit
	}

	piParams.AddExpand("data.latest_charge")
	piParams.AddExpand("data.invoice")
	piIter := sc.PaymentIntents.List(piParams)

	invoices := make([]map[string]any, 0)
	var lifetimePaid int64
	var lifetimePaidByCurrency = map[string]int64{}

	for piIter.Next() {
		pi := piIter.PaymentIntent()
		if pi.Status != stripe.PaymentIntentStatusSucceeded {
			continue
		}

		receiptURL := ""
		buyerName := "Guest"
		description := pi.Description

		if pi.Invoice != nil {
			if pi.Invoice.Lines != nil && len(pi.Invoice.Lines.Data) > 0 {
				description = pi.Invoice.Lines.Data[0].Description
			}
			if pi.Invoice.CustomerName != "" {
				buyerName = pi.Invoice.CustomerName
			} else if pi.Invoice.CustomerEmail != "" {
				buyerName = pi.Invoice.CustomerEmail
			}
		}

		if description == "" || description == "Subscription creation" {
			description = "Trial Session Payment"
		}

		// Try to get receipt and buyer from latest charge
		if pi.LatestCharge != nil {
			receiptURL = pi.LatestCharge.ReceiptURL
			if pi.LatestCharge.BillingDetails != nil && pi.LatestCharge.BillingDetails.Name != "" {
				buyerName = pi.LatestCharge.BillingDetails.Name
			} else if pi.LatestCharge.BillingDetails != nil && pi.LatestCharge.BillingDetails.Email != "" {
				buyerName = pi.LatestCharge.BillingDetails.Email
			}
		}

		cur := string(pi.Currency)
		invoices = append(invoices, map[string]any{
			"id":                 pi.ID,
			"number":             pi.ID[3:11], // short reference
			"amount_due":         pi.Amount,
			"amount_paid":        pi.AmountReceived,
			"currency":           cur,
			"status":             "paid",
			"created":            pi.Created,
			"hosted_invoice_url": receiptURL,
			"invoice_pdf":        receiptURL,
			"description":        description,
			"buyer_name":         buyerName,
			"metadata":           pi.Metadata,
		})

		lifetimePaidByCurrency[cur] += pi.AmountReceived
		lifetimePaid += pi.AmountReceived
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"invoices": invoices,
		"stats": map[string]any{
			"outstandingSGD":  int64(0),
			"lifetimePaidSGD": lifetimePaidByCurrency["sgd"],
			"lifetimePaidTotal":    lifetimePaid,
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
		_ = h.svc.UpdatePayments(r.Context(), studioID, token.StripeUserID, "", "", "", s.SubscriptionTier)
	}

	// Redirect back to frontend
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	http.Redirect(w, r, fmt.Sprintf("%s/admin/studios/%s/settings?tab=integrations", frontendURL, studioID), http.StatusTemporaryRedirect)
}

func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	plans, err := h.svc.ListPlans(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list plans")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"plans": plans})
}

type updatePlanReq struct {
	PriceSGD *int      `json:"priceSgd"`
	Features *[]string `json:"features"`
	IsActive *bool     `json:"isActive"`
}

func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_plan_id", "invalid plan id")
		return
	}
	var req updatePlanReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	
	err = h.svc.UpdatePlan(r.Context(), studioID, planID, UpdatePlanInput{
		PriceSGD: req.PriceSGD,
		Features: req.Features,
		IsActive: req.IsActive,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "plan not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to update plan")
		return
	}
	
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}

	// Get authenticated user claims
	claims := identity.MustClaims(r.Context())
	if claims.StudioID == nil || claims.StudioID.String() != studioID.String() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to delete this studio")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request")
		return
	}

	// Verify email matches
	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil || studio == nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "studio not found")
		return
	}

	if studio.ContactEmail != req.Email {
		httpx.WriteError(w, http.StatusBadRequest, "email_mismatch", "email does not match studio contact email")
		return
	}

	// Delete the studio and all associated data
	_, err = h.svc.repo.Pool().Exec(r.Context(), `
		DELETE FROM studios WHERE id = $1
	`, studioID)

	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", "failed to delete studio")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
