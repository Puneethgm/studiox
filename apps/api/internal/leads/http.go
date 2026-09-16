package leads

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	svc        *Service
	publicBase string
}

func NewHandler(svc *Service, cfg config.Config) *Handler {
	return &Handler{svc: svc, publicBase: cfg.PublicFormBaseURL}
}

// AdminRoutes are mounted UNDER /admin/studios/{studioId} and require an
// authenticated user. Authorization (super_admin OR matching studio_admin) is
// enforced by resolveStudioID below.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/campaigns", h.listCampaigns)
	r.Post("/campaigns", h.createCampaign)
	r.Get("/campaigns/{id}", h.getCampaign)
	r.Patch("/campaigns/{id}", h.patchCampaign)

	r.Get("/leads", h.listLeads)
	r.Get("/leads/stats", h.leadStats)
	r.Get("/analytics", h.getAnalytics)
	r.Get("/leads/sheets-settings", h.getSheetsSettings)
	r.Post("/leads/sheets-settings", h.saveSheetsSettings)
	r.Get("/leads/external-sheet-settings", h.getExternalLeadsSheetSettings)
	r.Post("/leads/external-sheet-settings", h.saveExternalLeadsSheetSettings)
	r.Post("/leads/import", h.importLeads)
	r.Get("/leads/sources", h.listUniqueSources)
	r.Get("/leads/{id}", h.getLead)
	r.Patch("/leads/{id}", h.patchLead)
	r.Patch("/leads/{id}/dnd", h.setLeadDND)
}

// PublicRoutes are unauthenticated.
//
//	GET  /public/studios/{studioSlug}/campaigns/{campaignSlug}
//	POST /public/studios/{studioSlug}/campaigns/{campaignSlug}/leads
//	POST /public/studios/{studioSlug}/trial-signup
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/public/studios/{studioSlug}/campaigns/{campaignSlug}", h.publicCampaign)
	r.Post("/public/studios/{studioSlug}/campaigns/{campaignSlug}/leads", h.publicSubmit)
	r.Patch("/public/leads/{leadId}/trial-slot", h.publicBookSlot)
	r.Post("/public/studios/{studioSlug}/trial-signup", h.publicTrialSignup)
}

// resolveStudioID returns the effective studio_id for the request and
// short-circuits with the right error response if forbidden.
func (h *Handler) resolveStudioID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	c := identity.MustClaims(r.Context())

	// Super admins can access any studio via URL param
	if c.IsSuper() {
		studioIDStr := chi.URLParam(r, "studioId")
		if studioIDStr == "" {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "studioId parameter required")
			return uuid.Nil, false
		}
		studioID, err := uuid.Parse(studioIDStr)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId format")
			return uuid.Nil, false
		}
		return studioID, true
	}

	// Studio admins use their bound studio
	if c.StudioID == nil {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
		return uuid.Nil, false
	}
	return *c.StudioID, true
}

// ----- admin: campaigns -----

type createCampaignReq struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	FitnessPlans []string `json:"fitnessPlans"`
}

type campaignRes struct {
	Campaign
	ShareURL string `json:"shareUrl"`
}

// createCampaign godoc
//
//	@Summary		Create a campaign
//	@Description	Creates a new campaign (slug, name, description, fitness plans) for the resolved studio and returns it with its public share URL. Mounted at both the super-admin prefix and the studio-scoped prefix. resolveStudioID reads `studioId` only from the URL path; the super-admin prefix below has no `{studioId}` path segment, so calling it there currently returns 400 studioId parameter required — the studio-scoped path is the only way this works today.
//	@Tags			Campaigns
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string				true	"Studio ID"
//	@Param			body		body		createCampaignReq	true	"Campaign fields"
//	@Success		201			{object}	campaignRes
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed or missing/invalid studioId"
//	@Failure		409			{object}	httpx.ErrorResponse	"slug already in use"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/campaigns [post]
//	@Router			/api/v1/studios/{studioId}/campaigns [post]
func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	c := identity.MustClaims(r.Context())
	var req createCampaignReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	camp, errs, err := h.svc.CreateCampaign(r.Context(), studioID, c.UserID, CreateCampaignInput{
		Slug:         req.Slug,
		Name:         req.Name,
		Description:  req.Description,
		FitnessPlans: req.FitnessPlans,
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrSlugTaken) {
			httpx.WriteError(w, http.StatusConflict, "slug_taken", "slug already in use")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	// Re-fetch to include studio name/slug for shareUrl rendering.
	full, _ := h.svc.GetCampaign(r.Context(), studioID, camp.ID)
	if full == nil {
		full = camp
	}
	httpx.JSON(w, http.StatusCreated, h.toCampaignRes(full))
}

// listCampaigns godoc
//
//	@Summary		List campaigns
//	@Description	Lists campaigns for the resolved studio, paginated via `limit`/`offset` query params (default limit 50), each with its public share URL. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Campaigns
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			limit		query		int		false	"Max campaigns to return (default 50)"
//	@Param			offset		query		int		false	"Offset for pagination (default 0)"
//	@Success		200			{object}	map[string]interface{}	"{campaigns: campaignRes[], total: int}"
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/campaigns [get]
//	@Router			/api/v1/studios/{studioId}/campaigns [get]
func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	limitVal := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 {
			limitVal = val
		}
	}
	offsetVal := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if val, err := strconv.Atoi(oStr); err == nil && val >= 0 {
			offsetVal = val
		}
	}

	list, total, err := h.svc.ListCampaigns(r.Context(), studioID, limitVal, offsetVal)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	out := make([]campaignRes, 0, len(list))
	for i := range list {
		out = append(out, h.toCampaignRes(&list[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"campaigns": out,
		"total":     total,
	})
}

// getCampaign godoc
//
//	@Summary		Get a campaign
//	@Description	Fetches a single campaign by ID for the resolved studio, including its public share URL. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Campaigns
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			id			path		string	true	"Campaign ID"
//	@Success		200			{object}	campaignRes
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId or invalid campaign id"
//	@Failure		404			{object}	httpx.ErrorResponse	"campaign not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/campaigns/{id} [get]
//	@Router			/api/v1/studios/{studioId}/campaigns/{id} [get]
func (h *Handler) getCampaign(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	camp, err := h.svc.GetCampaign(r.Context(), studioID, id)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, h.toCampaignRes(camp))
}

type patchCampaignReq struct {
	Active       *bool     `json:"active"`
	FitnessPlans *[]string `json:"fitnessPlans"`
}

// patchCampaign godoc
//
//	@Summary		Update a campaign
//	@Description	Partially updates a campaign: `fitnessPlans` (replaces the list) and/or `active` (activate/deactivate). Either or both fields may be supplied; returns the campaign after applying the requested changes. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Campaigns
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string				true	"Studio ID"
//	@Param			id			path		string				true	"Campaign ID"
//	@Param			body		body		patchCampaignReq	true	"Fields to update"
//	@Success		200			{object}	campaignRes
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed, missing/invalid studioId, or invalid campaign id"
//	@Failure		404			{object}	httpx.ErrorResponse	"campaign not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/campaigns/{id} [patch]
//	@Router			/api/v1/studios/{studioId}/campaigns/{id} [patch]
func (h *Handler) patchCampaign(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req patchCampaignReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.FitnessPlans != nil {
		camp, errs, err := h.svc.UpdateCampaignFitnessPlans(r.Context(), studioID, id, *req.FitnessPlans)
		if errs != nil {
			httpx.WriteValidationError(w, errs)
			return
		}
		if err != nil {
			if errors.Is(err, ErrCampaignNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if req.Active == nil {
			httpx.JSON(w, http.StatusOK, h.toCampaignRes(camp))
			return
		}
	}
	if req.Active != nil {
		if err := h.svc.SetCampaignActive(r.Context(), studioID, id, *req.Active); err != nil {
			if errors.Is(err, ErrCampaignNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	camp, err := h.svc.GetCampaign(r.Context(), studioID, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, h.toCampaignRes(camp))
}

// ----- admin: leads -----

// listLeads godoc
//
//	@Summary		List leads
//	@Description	Lists leads for the resolved studio with extensive optional filtering (campaign, status, attempt count, source, date range/duration, hot lead, contact made, trial purchased) plus free-text search and pagination. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId		path		string	true	"Studio ID"
//	@Param			campaignId		query		string	false	"Filter by campaign ID"
//	@Param			status			query		string	false	"Filter by a single lead status"
//	@Param			statuses		query		string	false	"Filter by comma-separated lead statuses"
//	@Param			maxAttempts		query		int		false	"Filter by maximum contact attempts"
//	@Param			source			query		string	false	"Filter by lead source"
//	@Param			startDate		query		string	false	"Filter: created on/after this date"
//	@Param			endDate			query		string	false	"Filter: created on/before this date"
//	@Param			duration		query		string	false	"Filter: created within the last N days, e.g. '30d'"
//	@Param			hotLead			query		bool	false	"Filter by hot-lead flag"
//	@Param			contactMade		query		bool	false	"Filter by contact-made flag"
//	@Param			trialPurchased	query		bool	false	"Filter by trial-purchased flag"
//	@Param			search			query		string	false	"Free-text search"
//	@Param			limit			query		int		false	"Max leads to return"
//	@Param			offset			query		int		false	"Offset for pagination"
//	@Success		200				{object}	map[string]interface{}	"{leads: Lead[], total: int}"
//	@Failure		400				{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500				{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads [get]
//	@Router			/api/v1/studios/{studioId}/leads [get]
func (h *Handler) listLeads(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()

	f := ListLeadsFilter{}
	if v := q.Get("campaignId"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			f.CampaignID = &id
		}
	}
	if v := q.Get("status"); v != "" {
		s := LeadStatus(v)
		if s.Valid() {
			f.Status = &s
		}
	}
	if v := q.Get("statuses"); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			s := LeadStatus(p)
			if s.Valid() {
				f.Statuses = append(f.Statuses, s)
			}
		}
	}
	if v := q.Get("maxAttempts"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			f.MaxAttempts = &n
		}
	}
	if v := q.Get("source"); v != "" {
		f.Source = v
	}
	if v := q.Get("startDate"); v != "" {
		f.StartDate = v
	}
	if v := q.Get("endDate"); v != "" {
		f.EndDate = v
	}
	if v := q.Get("duration"); v != "" {
		v = strings.TrimSuffix(v, "d")
		n, err := strconv.Atoi(v)
		if err == nil {
			f.DurationDays = n
		}
	}
	if v := q.Get("hotLead"); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			f.HotLead = &b
		}
	}
	if v := q.Get("contactMade"); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			f.ContactMade = &b
		}
	}
	if v := q.Get("trialPurchased"); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			f.TrialPurchased = &b
		}
	}
	if v := q.Get("limit"); v != "" {
		n, _ := strconv.Atoi(v)
		f.Limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, _ := strconv.Atoi(v)
		f.Offset = n
	}
	if v := q.Get("search"); v != "" {
		f.Search = v
	}

	list, total, err := h.svc.ListLeads(r.Context(), studioID, f)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"leads": list,
		"total": total,
	})
}

// leadStats godoc
//
//	@Summary		Lead stats
//	@Description	Returns aggregate lead counts (total + breakdown by status) for the resolved studio, for pipeline widgets. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	LeadStats
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/stats [get]
//	@Router			/api/v1/studios/{studioId}/leads/stats [get]
func (h *Handler) leadStats(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	stats, err := h.svc.Stats(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, stats)
}

// getAnalytics godoc
//
//	@Summary		Lead analytics summary
//	@Description	Returns an analytics summary for the resolved studio over a time window. `duration` accepts '15d'/'30d'/'90d'/'365d' or any 'Nd' value; an explicit `startDate`/`endDate` range can also be supplied; omitting duration returns all-time. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			duration	query		string	false	"Time window, e.g. '30d' (default: all-time)"
//	@Param			startDate	query		string	false	"Explicit range start date"
//	@Param			endDate		query		string	false	"Explicit range end date"
//	@Success		200			{object}	AnalyticsSummary
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/analytics [get]
//	@Router			/api/v1/studios/{studioId}/analytics [get]
func (h *Handler) getAnalytics(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}

	durationStr := r.URL.Query().Get("duration")
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var durationDays int
	if strings.HasSuffix(durationStr, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(durationStr, "d"))
		if err == nil && days > 0 {
			durationDays = days
		}
	} else {
		switch durationStr {
		case "15d":
			durationDays = 15
		case "30d":
			durationDays = 30
		case "90d":
			durationDays = 90
		case "365d":
			durationDays = 365
		default:
			durationDays = 0 // all-time
		}
	}

	summary, err := h.svc.GetAnalytics(r.Context(), studioID, durationDays, startDate, endDate)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

// getLead godoc
//
//	@Summary		Get a lead
//	@Description	Fetches a single lead by ID for the resolved studio. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			id			path		string	true	"Lead ID"
//	@Success		200			{object}	Lead
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId or invalid lead id"
//	@Failure		404			{object}	httpx.ErrorResponse	"lead not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/{id} [get]
//	@Router			/api/v1/studios/{studioId}/leads/{id} [get]
func (h *Handler) getLead(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	l, err := h.svc.GetLead(r.Context(), studioID, id)
	if err != nil {
		if errors.Is(err, ErrLeadNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "lead not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

type patchLeadReq struct {
	Status         *LeadStatus `json:"status"`
	Notes          *string     `json:"notes"`
	ContactMade    *bool       `json:"contactMade"`
	HotLead        *bool       `json:"hotLead"`
	TrialPurchased *bool       `json:"trialPurchased"`
	FitnessPlan    *string     `json:"fitnessPlan"`
	FirstName      *string     `json:"firstName"`
	LastName       *string     `json:"lastName"`
	AssignedTo     *string     `json:"assignedTo"`
	TrialAttended  *bool       `json:"trialAttended"`
	MemberSold     *bool       `json:"memberSold"`
	MonthlyFee     *float64    `json:"monthlyFee"`
	Currency       *string     `json:"currency"`
	Offer          *string     `json:"offer"`
	FurtherNotes   *string     `json:"furtherNotes"`
}

type setLeadDNDReq struct {
	Enabled bool `json:"enabled"`
}

// setLeadDND godoc
//
//	@Summary		Set lead Do Not Disturb
//	@Description	Toggles Do Not Disturb for a lead. Turning it on silences all automated messaging (autocontact follow-ups, AI/decision-tree replies) and cancels every already-queued send for this lead; the lead's pipeline status is left untouched. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string			true	"Studio ID"
//	@Param			id			path		string			true	"Lead ID"
//	@Param			body		body		setLeadDNDReq	true	"DND state"
//	@Success		200			{object}	Lead
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId or invalid lead id"
//	@Failure		404			{object}	httpx.ErrorResponse	"lead not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/{id}/dnd [patch]
//	@Router			/api/v1/studios/{studioId}/leads/{id}/dnd [patch]
func (h *Handler) setLeadDND(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req setLeadDNDReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	lead, err := h.svc.SetDND(r.Context(), studioID, id, req.Enabled)
	if err != nil {
		if errors.Is(err, ErrLeadNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "lead not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, lead)
}

// patchLead godoc
//
//	@Summary		Update a lead
//	@Description	Partially updates a lead (status, notes, contact/hot-lead/trial flags, fitness plan, name, assignment, trial attendance, member sold, monthly fee, currency, offer, further notes). Unset fields keep their current value; returns the lead after applying the requested changes. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string			true	"Studio ID"
//	@Param			id			path		string			true	"Lead ID"
//	@Param			body		body		patchLeadReq	true	"Fields to update"
//	@Success		200			{object}	Lead
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed, missing/invalid studioId, or invalid lead id"
//	@Failure		404			{object}	httpx.ErrorResponse	"lead not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/{id} [patch]
//	@Router			/api/v1/studios/{studioId}/leads/{id} [patch]
func (h *Handler) patchLead(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req patchLeadReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	current, err := h.svc.GetLead(r.Context(), studioID, id)
	if err != nil {
		if errors.Is(err, ErrLeadNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "lead not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	status := current.Status
	notes := current.Notes
	contactMade := current.ContactMade
	hotLead := current.HotLead
	trialPurchased := current.TrialPurchased
	fitnessPlan := current.FitnessPlan
	firstName := current.FirstName
	lastName := current.LastName
	assignedTo := current.AssignedTo
	trialAttended := current.TrialAttended
	memberSold := current.MemberSold
	monthlyFee := current.MonthlyFee
	currency := current.Currency
	offer := current.Offer
	furtherNotes := current.FurtherNotes

	if req.Status != nil {
		status = *req.Status
	}
	if req.Notes != nil {
		notes = *req.Notes
	}
	if req.ContactMade != nil {
		contactMade = *req.ContactMade
	}
	if req.HotLead != nil {
		hotLead = *req.HotLead
	}
	if req.TrialPurchased != nil {
		trialPurchased = *req.TrialPurchased
	}
	if req.FitnessPlan != nil {
		fitnessPlan = *req.FitnessPlan
	}
	if req.FirstName != nil {
		firstName = *req.FirstName
	}
	if req.LastName != nil {
		lastName = *req.LastName
	}
	if req.AssignedTo != nil {
		assignedTo = *req.AssignedTo
	}
	if req.TrialAttended != nil {
		trialAttended = *req.TrialAttended
	}
	if req.MemberSold != nil {
		memberSold = *req.MemberSold
	}
	if req.MonthlyFee != nil {
		monthlyFee = *req.MonthlyFee
	}
	if req.Currency != nil {
		currency = *req.Currency
	}
	if req.Offer != nil {
		offer = *req.Offer
	}
	if req.FurtherNotes != nil {
		furtherNotes = *req.FurtherNotes
	}

	if err := h.svc.UpdateLead(r.Context(), studioID, id, status, currency, notes, contactMade, hotLead, trialPurchased, firstName, lastName, fitnessPlan, assignedTo, trialAttended, memberSold, monthlyFee, offer, furtherNotes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	updated, err := h.svc.GetLead(r.Context(), studioID, id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

// ----- public -----

type publicCampaignRes struct {
	StudioSlug   string   `json:"studioSlug"`
	StudioName   string   `json:"studioName"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	FitnessPlans []string `json:"fitnessPlans"`
}

// publicCampaign godoc
//
//	@Summary		Get a public campaign's lead-capture form info
//	@Description	Public, unauthenticated endpoint that returns a campaign's public-facing details (studio branding info, name, description, fitness plans) for rendering the lead-capture form at /l/{studioSlug}/{campaignSlug}.
//	@Tags			Leads (Public)
//	@Produce		json
//	@Param			studioSlug		path		string	true	"Studio slug"
//	@Param			campaignSlug	path		string	true	"Campaign slug"
//	@Success		200				{object}	publicCampaignRes
//	@Failure		404				{object}	httpx.ErrorResponse	"campaign not found"
//	@Failure		500				{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/studios/{studioSlug}/campaigns/{campaignSlug} [get]
func (h *Handler) publicCampaign(w http.ResponseWriter, r *http.Request) {
	studioSlug := chi.URLParam(r, "studioSlug")
	campaignSlug := chi.URLParam(r, "campaignSlug")
	c, err := h.svc.GetPublicCampaign(r.Context(), studioSlug, campaignSlug)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, publicCampaignRes{
		StudioSlug:   c.StudioSlug,
		StudioName:   c.StudioName,
		Slug:         c.Slug,
		Name:         c.Name,
		Description:  c.Description,
		FitnessPlans: c.FitnessPlans,
	})
}

type publicSubmitReq struct {
	Name        string `json:"name"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	FitnessPlan string `json:"fitnessPlan"`
	Goals       string `json:"goals"`
}

// publicSubmit godoc
//
//	@Summary		Submit a lead via the public campaign form
//	@Description	Public, unauthenticated endpoint used by the public lead-capture form to submit a new lead for an active campaign. Captures referrer, user agent, and client IP server-side. Field lengths are validated (name/email <=255, firstName/lastName <=100, phone <=30, goals <=2000).
//	@Tags			Leads (Public)
//	@Accept			json
//	@Produce		json
//	@Param			studioSlug		path		string				true	"Studio slug"
//	@Param			campaignSlug	path		string				true	"Campaign slug"
//	@Param			body			body		publicSubmitReq		true	"Lead submission"
//	@Success		201				{object}	map[string]interface{}	"{id, studioName, campaignName}"
//	@Failure		400				{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		404				{object}	httpx.ErrorResponse	"campaign not found or inactive"
//	@Failure		500				{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/studios/{studioSlug}/campaigns/{campaignSlug}/leads [post]
func (h *Handler) publicSubmit(w http.ResponseWriter, r *http.Request) {
	studioSlug := chi.URLParam(r, "studioSlug")
	campaignSlug := chi.URLParam(r, "campaignSlug")
	var req publicSubmitReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	valErrs := map[string]string{}
	if len(req.Name) > 255 {
		valErrs["name"] = "must be 255 characters or less"
	}
	if len(req.FirstName) > 100 {
		valErrs["firstName"] = "must be 100 characters or less"
	}
	if len(req.LastName) > 100 {
		valErrs["lastName"] = "must be 100 characters or less"
	}
	if len(req.Email) > 255 {
		valErrs["email"] = "must be 255 characters or less"
	}
	if len(req.Phone) > 30 {
		valErrs["phone"] = "must be 30 characters or less"
	}
	if len(req.Goals) > 2000 {
		valErrs["goals"] = "must be 2000 characters or less"
	}
	if len(valErrs) > 0 {
		httpx.WriteValidationError(w, valErrs)
		return
	}
	lead, errs, err := h.svc.SubmitPublicLead(r.Context(), SubmitLeadInput{
		StudioSlug:   studioSlug,
		CampaignSlug: campaignSlug,
		Name:         req.Name,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        req.Email,
		Phone:        req.Phone,
		FitnessPlan:  req.FitnessPlan,
		Goals:        req.Goals,
		Referrer:     r.Header.Get("Referer"),
		UserAgent:    r.UserAgent(),
		IPAddress:    httpx.ClientIP(r),
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found or inactive")
			return
		}
		slog.Error("publicSubmit failed", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":           lead.ID,
		"studioName":   lead.StudioName,
		"campaignName": lead.CampaignName,
	})
}

type publicTrialSignupReq struct {
	FullName    string `json:"fullName"`
	Phone       string `json:"phone"`
	Gender      string `json:"gender"`
	DateOfBirth string `json:"dateOfBirth"`
}

// publicTrialSignup godoc
//
//	@Summary		Submit a public trial signup
//	@Description	Public, unauthenticated endpoint for a visitor to sign up for a trial directly against a studio (not tied to a specific campaign slug in the URL). Captures referrer, user agent, and client IP server-side. Field lengths are validated (fullName <=255, phone <=30).
//	@Tags			Leads (Public)
//	@Accept			json
//	@Produce		json
//	@Param			studioSlug	path		string					true	"Studio slug"
//	@Param			body		body		publicTrialSignupReq	true	"Trial signup"
//	@Success		201			{object}	map[string]interface{}	"{leadId}"
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		404			{object}	httpx.ErrorResponse	"studio has no trial link configured"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/studios/{studioSlug}/trial-signup [post]
func (h *Handler) publicTrialSignup(w http.ResponseWriter, r *http.Request) {
	studioSlug := chi.URLParam(r, "studioSlug")
	var req publicTrialSignupReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	valErrs := map[string]string{}
	if len(req.FullName) > 255 {
		valErrs["fullName"] = "must be 255 characters or less"
	}
	if len(req.Phone) > 30 {
		valErrs["phone"] = "must be 30 characters or less"
	}
	if len(valErrs) > 0 {
		httpx.WriteValidationError(w, valErrs)
		return
	}
	lead, errs, err := h.svc.SubmitTrialSignup(r.Context(), TrialSignupInput{
		StudioSlug:  studioSlug,
		FullName:    req.FullName,
		Phone:       req.Phone,
		Gender:      req.Gender,
		DateOfBirth: req.DateOfBirth,
		Referrer:    r.Header.Get("Referer"),
		UserAgent:   r.UserAgent(),
		IPAddress:   httpx.ClientIP(r),
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "this studio doesn't have a trial link set up yet")
			return
		}
		slog.Error("publicTrialSignup failed", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"leadId": lead.ID})
}

// publicBookSlot godoc
//
//	@Summary		Book a trial slot for a lead
//	@Description	Public, unauthenticated endpoint that lets a lead pick/confirm a trial time slot after signup. `slot` is required.
//	@Tags			Leads (Public)
//	@Accept			json
//	@Produce		json
//	@Param			leadId	path		string					true	"Lead ID"
//	@Param			body	body		object{slot=string}		true	"Trial slot selection"
//	@Success		200		{object}	map[string]interface{}	"{status: \"ok\"}"
//	@Failure		400		{object}	httpx.ErrorResponse	"invalid lead id or missing slot"
//	@Failure		404		{object}	httpx.ErrorResponse	"lead not found"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/public/leads/{leadId}/trial-slot [patch]
func (h *Handler) publicBookSlot(w http.ResponseWriter, r *http.Request) {
	leadID, err := uuid.Parse(chi.URLParam(r, "leadId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_lead_id", "invalid lead id")
		return
	}
	var req struct {
		Slot string `json:"slot"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Slot == "" {
		httpx.WriteValidationError(w, map[string]string{"slot": "required"})
		return
	}
	err = h.svc.BookTrialSlot(r.Context(), leadID, req.Slot)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "lead not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ----- helpers -----

func (h *Handler) toCampaignRes(c *Campaign) campaignRes {
	share := h.publicBase + "/l/" + c.StudioSlug + "/" + c.Slug
	return campaignRes{Campaign: *c, ShareURL: share}
}

type saveSheetsSettingsReq struct {
	SpreadsheetID string `json:"spreadsheetId"`
	TabName       string `json:"tabName"`
	Active        bool   `json:"active"`
}

// getSheetsSettings godoc
//
//	@Summary		Get Google Sheets sync settings
//	@Description	Returns the resolved studio's Google Sheets outbox sync settings (spreadsheet ID, tab name, active flag). If none have been saved yet, returns a default {spreadsheetId: "", tabName: "Leads", active: false}. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	StudioSheetsSettings
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/sheets-settings [get]
//	@Router			/api/v1/studios/{studioId}/leads/sheets-settings [get]
func (h *Handler) getSheetsSettings(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	settings, err := h.svc.GetSheetsSettings(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if settings == nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"spreadsheetId": "", "tabName": "Leads", "active": false})
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

// saveSheetsSettings godoc
//
//	@Summary		Save Google Sheets sync settings
//	@Description	Creates or updates the resolved studio's Google Sheets outbox sync settings. `spreadsheetId` is required. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string					true	"Studio ID"
//	@Param			body		body		saveSheetsSettingsReq	true	"Sheets settings"
//	@Success		200			{object}	StudioSheetsSettings
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed or missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/sheets-settings [post]
//	@Router			/api/v1/studios/{studioId}/leads/sheets-settings [post]
func (h *Handler) saveSheetsSettings(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	var req saveSheetsSettingsReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.SpreadsheetID == "" {
		httpx.WriteValidationError(w, map[string]string{"spreadsheetId": "required"})
		return
	}
	settings, err := h.svc.SaveSheetsSettings(r.Context(), studioID, req.SpreadsheetID, req.TabName, req.Active)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

type saveExternalLeadsSheetSettingsReq struct {
	SpreadsheetID   string `json:"spreadsheetId"`
	TabName         string `json:"tabName"`
	NameColumn      string `json:"nameColumn"`
	FirstNameColumn string `json:"firstNameColumn"`
	LastNameColumn  string `json:"lastNameColumn"`
	EmailColumn     string `json:"emailColumn"`
	PhoneColumn     string `json:"phoneColumn"`
	SourceColumn    string `json:"sourceColumn"`
	NotesColumn     string `json:"notesColumn"`
	DateColumn      string `json:"dateColumn"`
	HotLeadColumn   string `json:"hotLeadColumn"`
	TrialPurchasedColumn string `json:"trialPurchasedColumn"`
	ContinueAIAfterGreeting bool `json:"continueAiAfterGreeting"`
	AutoContactEnabled *bool `json:"autoContactEnabled"`
	Active          bool   `json:"active"`
}

// getExternalLeadsSheetSettings godoc
//
//	@Summary		Get external leads sheet import settings
//	@Description	Returns the resolved studio's settings for importing leads from an external Google Sheet (spreadsheet ID, tab name, column mappings, and import behavior flags). If none have been saved yet, returns sensible defaults. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	ExternalLeadsSheetSettings
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/external-sheet-settings [get]
//	@Router			/api/v1/studios/{studioId}/leads/external-sheet-settings [get]
func (h *Handler) getExternalLeadsSheetSettings(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	settings, err := h.svc.GetExternalLeadsSheetSettings(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if settings == nil {
		httpx.JSON(w, http.StatusOK, map[string]any{
			"spreadsheetId": "", "tabName": "Sheet1",
			"nameColumn": "", "firstNameColumn": "A", "lastNameColumn": "B",
			"emailColumn": "C", "phoneColumn": "D", "sourceColumn": "", "notesColumn": "", "dateColumn": "",
			"hotLeadColumn": "", "trialPurchasedColumn": "",
			"continueAiAfterGreeting": true,
			"autoContactEnabled": true,
			"active": false,
		})
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

// saveExternalLeadsSheetSettings godoc
//
//	@Summary		Save external leads sheet import settings
//	@Description	Creates or updates the resolved studio's settings for importing leads from an external Google Sheet, including column mappings and import behavior flags (`autoContactEnabled` defaults to true when omitted). `spreadsheetId` is required. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string								true	"Studio ID"
//	@Param			body		body		saveExternalLeadsSheetSettingsReq	true	"External sheet settings"
//	@Success		200			{object}	ExternalLeadsSheetSettings
//	@Failure		400			{object}	httpx.ErrorResponse	"validation failed or missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/external-sheet-settings [post]
//	@Router			/api/v1/studios/{studioId}/leads/external-sheet-settings [post]
func (h *Handler) saveExternalLeadsSheetSettings(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	var req saveExternalLeadsSheetSettingsReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.SpreadsheetID == "" {
		httpx.WriteValidationError(w, map[string]string{"spreadsheetId": "required"})
		return
	}
	autoContactEnabled := true
	if req.AutoContactEnabled != nil {
		autoContactEnabled = *req.AutoContactEnabled
	}
	settings, err := h.svc.SaveExternalLeadsSheetSettings(r.Context(), studioID, ExternalLeadsSheetSettings{
		SpreadsheetID:   req.SpreadsheetID,
		TabName:         req.TabName,
		NameColumn:      req.NameColumn,
		FirstNameColumn: req.FirstNameColumn,
		LastNameColumn:  req.LastNameColumn,
		EmailColumn:     req.EmailColumn,
		PhoneColumn:     req.PhoneColumn,
		SourceColumn:    req.SourceColumn,
		NotesColumn:     req.NotesColumn,
		DateColumn:      req.DateColumn,
		HotLeadColumn:   req.HotLeadColumn,
		TrialPurchasedColumn: req.TrialPurchasedColumn,
		ContinueAIAfterGreeting: req.ContinueAIAfterGreeting,
		AutoContactEnabled: autoContactEnabled,
		Active:          req.Active,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, settings)
}

// importLeads godoc
//
//	@Summary		Import leads from a file
//	@Description	Bulk-imports leads for the resolved studio from an uploaded .csv, .xlsx, or .xls file (multipart form, 10MB max), assigning them to the given default campaign. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string					true	"Studio ID"
//	@Param			file		formData	file					true	"CSV or Excel (.xlsx/.xls) file of leads"
//	@Param			campaignId	formData	string					true	"Default campaign ID to assign imported leads to"
//	@Success		200			{object}	map[string]interface{}	"{imported: int, message: string}"
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId, missing/invalid campaignId, malformed multipart form, unsupported file format, or unparseable file"
//	@Failure		500			{object}	httpx.ErrorResponse	"import failed"
//	@Router			/api/v1/admin/leads/import [post]
//	@Router			/api/v1/studios/{studioId}/leads/import [post]
func (h *Handler) importLeads(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	campaignIDStr := r.FormValue("campaignId")
	if campaignIDStr == "" {
		httpx.WriteValidationError(w, map[string]string{"campaignId": "default campaign is required"})
		return
	}
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		httpx.WriteValidationError(w, map[string]string{"campaignId": "invalid campaign ID"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var rows [][]string

	if ext == ".csv" {
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		rows, err = reader.ReadAll()
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_csv", fmt.Sprintf("failed to parse CSV: %v", err))
			return
		}
	} else if ext == ".xlsx" || ext == ".xls" {
		f, err := excelize.OpenReader(file)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to open Excel: %v", err))
			return
		}
		sheetName := f.GetSheetName(0)
		if sheetName == "" {
			sheetName = "Sheet1"
		}
		rows, err = f.GetRows(sheetName)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to read Excel sheet: %v", err))
			return
		}
	} else {
		httpx.WriteError(w, http.StatusBadRequest, "unsupported_format", "file must be a .csv or .xlsx file")
		return
	}

	count, err := h.svc.ImportLeads(r.Context(), studioID, campaignID, rows)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"imported": count,
		"message":  fmt.Sprintf("Successfully imported %d leads", count),
	})
}

// listUniqueSources godoc
//
//	@Summary		List unique lead sources
//	@Description	Returns the distinct `source` values seen across the resolved studio's leads, for populating filter dropdowns. Mounted at both the super-admin prefix and the studio-scoped prefix; see resolveStudioID note on createCampaign above regarding studioId resolution at the super-admin prefix.
//	@Tags			Leads
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{array}		string
//	@Failure		400			{object}	httpx.ErrorResponse	"missing/invalid studioId"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/admin/leads/sources [get]
//	@Router			/api/v1/studios/{studioId}/leads/sources [get]
func (h *Handler) listUniqueSources(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	sources, err := h.svc.GetUniqueSources(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, sources)
}
