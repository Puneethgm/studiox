package studios

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// AdminRoutes are super-admin only — only the platform owner manages studios.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Use(identity.RequireRole(identity.RoleSuperAdmin))
	r.Get("/studios", h.list)
	r.Post("/studios", h.create)
	r.Get("/studios/{id}", h.get)
	r.Patch("/studios/{id}", h.update)
}

// SelfRoutes are for any authenticated user. Studio admins use this to fetch
// their own studio for the settings page.
func (h *Handler) SelfRoutes(r chi.Router) {
	r.Get("/studios/{id}", h.getScoped)
	r.Patch("/studios/{id}", h.updateScoped)
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
	AdminEmail    string `json:"adminEmail"`
	AdminPassword string `json:"adminPassword"`
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
		ContactEmail:  req.ContactEmail,
		AdminEmail:    req.AdminEmail,
		AdminPassword: req.AdminPassword,
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
	Name         string `json:"name"`
	BrandColor   string `json:"brandColor"`
	LogoURL      string `json:"logoUrl"`
	ContactEmail string `json:"contactEmail"`
	Active               bool                 `json:"active"`
	AvailabilitySlots    []AvailabilitySlot   `json:"availabilitySlots"`
	AvailabilityTimezone string               `json:"availabilityTimezone"`
	GeminiAPIKey         string               `json:"geminiApiKey"`
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
	errs, err := h.svc.Update(r.Context(), id, UpdateStudioInput{
		Name:         req.Name,
		BrandColor:   req.BrandColor,
		LogoURL:      req.LogoURL,
		ContactEmail: req.ContactEmail,
		Active:               req.Active,
		AvailabilitySlots:    req.AvailabilitySlots,
		AvailabilityTimezone: req.AvailabilityTimezone,
		GeminiAPIKey:         req.GeminiAPIKey,
	})
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
	errs, err := h.svc.Update(r.Context(), studioID, UpdateStudioInput{
		Name:                 req.Name,
		BrandColor:           req.BrandColor,
		LogoURL:              req.LogoURL,
		ContactEmail:         req.ContactEmail,
		Active:               req.Active,
		AvailabilitySlots:    req.AvailabilitySlots,
		AvailabilityTimezone: req.AvailabilityTimezone,
		GeminiAPIKey:         req.GeminiAPIKey,
	})
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
