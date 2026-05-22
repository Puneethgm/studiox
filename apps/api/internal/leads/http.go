package leads

import (
	"encoding/csv"
	"errors"
	"fmt"
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
	r.Get("/leads/sheets-settings", h.getSheetsSettings)
	r.Post("/leads/sheets-settings", h.saveSheetsSettings)
	r.Post("/leads/import", h.importLeads)
	r.Get("/leads/{id}", h.getLead)
	r.Patch("/leads/{id}", h.patchLead)
}

// PublicRoutes are unauthenticated.
//
//	GET  /public/studios/{studioSlug}/campaigns/{campaignSlug}
//	POST /public/studios/{studioSlug}/campaigns/{campaignSlug}/leads
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/public/studios/{studioSlug}/campaigns/{campaignSlug}", h.publicCampaign)
	r.Post("/public/studios/{studioSlug}/campaigns/{campaignSlug}/leads", h.publicSubmit)
	r.Patch("/public/leads/{leadId}/trial-slot", h.publicBookSlot)
}

// resolveStudioID returns the effective studio_id for the request and
// short-circuits with the right error response if forbidden.
func (h *Handler) resolveStudioID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	c := identity.MustClaims(r.Context())
	if c.IsSuper() {
		pathID, err := uuid.Parse(chi.URLParam(r, "studioId"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_studio_id", "invalid studio id")
			return uuid.Nil, false
		}
		return pathID, true
	}
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

func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListCampaigns(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	out := make([]campaignRes, 0, len(list))
	for i := range list {
		out = append(out, h.toCampaignRes(&list[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"campaigns": out})
}

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
	FirstName      *string     `json:"firstName"`
	LastName       *string     `json:"lastName"`
}

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
	firstName := current.FirstName
	lastName := current.LastName

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
	if req.FirstName != nil {
		firstName = *req.FirstName
	}
	if req.LastName != nil {
		lastName = *req.LastName
	}

	if err := h.svc.UpdateLead(r.Context(), studioID, id, status, notes, contactMade, hotLead, trialPurchased, firstName, lastName); err != nil {
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
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	FitnessPlan string `json:"fitnessPlan"`
	Goals       string `json:"goals"`
}

func (h *Handler) publicSubmit(w http.ResponseWriter, r *http.Request) {
	studioSlug := chi.URLParam(r, "studioSlug")
	campaignSlug := chi.URLParam(r, "campaignSlug")
	var req publicSubmitReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	lead, errs, err := h.svc.SubmitPublicLead(r.Context(), SubmitLeadInput{
		StudioSlug:   studioSlug,
		CampaignSlug: campaignSlug,
		Name:         req.Name,
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
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":           lead.ID,
		"studioName":   lead.StudioName,
		"campaignName": lead.CampaignName,
	})
}

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
