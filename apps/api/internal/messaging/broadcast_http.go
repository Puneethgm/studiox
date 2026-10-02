package messaging

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/projectx/api/internal/platform/httpx"
)

// BroadcastRoutes are mounted under /api/v1/studios/{studioId}/messaging —
// Manual Actions' bulk-send feature: upload a contact sheet, compose one
// message, schedule it to the whole list. Read-vs-write split from the
// rest of AdminRoutes only for navigability; same auth/scoping as every
// other route in this file.
func (h *Handler) BroadcastRoutes(r chi.Router) {
	r.Post("/broadcasts/import", h.stageBroadcastImport)
	r.Post("/broadcasts/import/{importId}/confirm", h.confirmBroadcastImport)
	r.Get("/broadcasts/lists", h.listBroadcastLists)
	r.Get("/broadcasts/lists/{listId}", h.getBroadcastList)
	r.Delete("/broadcasts/lists/{listId}", h.deleteBroadcastList)
	r.Post("/broadcasts/campaigns", h.createBroadcastCampaign)
	r.Get("/broadcasts/campaigns", h.listBroadcastCampaigns)
	r.Get("/broadcasts/campaigns/{campaignId}/recipients", h.listBroadcastCampaignRecipients)
	r.Delete("/broadcasts/campaigns/{campaignId}", h.cancelBroadcastCampaign)
}

// stageBroadcastImport godoc
//
//	@Summary		Upload a contact sheet for a WhatsApp broadcast
//	@Description	Parses an uploaded .csv/.xlsx/.xls (Client Name, Phone, Email columns, fuzzy-matched) and stages it — nothing is saved yet. Returns how many rows are missing a country code on their phone number; if non-zero, call the confirm endpoint with defaultCountryCode to finish the import.
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			file		formData	file	true	"CSV or Excel file of contacts"
//	@Success		200			{object}	BroadcastImportPreview
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/import [post]
func (h *Handler) stageBroadcastImport(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
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

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var sheet [][]string
	switch ext {
	case ".csv":
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		sheet, err = reader.ReadAll()
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_csv", fmt.Sprintf("failed to parse CSV: %v", err))
			return
		}
	case ".xlsx", ".xls":
		f, err := excelize.OpenReader(file)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to open Excel: %v", err))
			return
		}
		sheetName := f.GetSheetName(0)
		if sheetName == "" {
			sheetName = "Sheet1"
		}
		sheet, err = f.GetRows(sheetName)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to read Excel sheet: %v", err))
			return
		}
	default:
		httpx.WriteError(w, http.StatusBadRequest, "unsupported_format", "file must be a .csv or .xlsx file")
		return
	}

	preview, err := h.svc.StageBroadcastImport(r.Context(), studioID, sheet)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, preview)
}

type confirmBroadcastImportReq struct {
	Name               string `json:"name"`
	DefaultCountryCode string `json:"defaultCountryCode"`
}

// confirmBroadcastImport godoc
//
//	@Summary		Finish a staged broadcast contact import
//	@Description	Applies defaultCountryCode (if given) to any staged row missing one, then saves the list.
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string						true	"Studio ID"
//	@Param			importId	path		string						true	"Import ID from the upload step"
//	@Param			body		body		confirmBroadcastImportReq	true	"List name and optional default country code"
//	@Success		201			{object}	BroadcastList
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/import/{importId}/confirm [post]
func (h *Handler) confirmBroadcastImport(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	importID, err := uuid.Parse(chi.URLParam(r, "importId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid import id")
		return
	}
	var req confirmBroadcastImportReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	list, err := h.svc.ConfirmBroadcastImport(r.Context(), studioID, importID, req.Name, req.DefaultCountryCode)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, list)
}

// listBroadcastLists godoc
//
//	@Summary		List broadcast contact lists
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]interface{}
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/lists [get]
func (h *Handler) listBroadcastLists(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	lists, err := h.svc.ListBroadcastLists(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"lists": lists})
}

// getBroadcastList godoc
//
//	@Summary		Get a broadcast list with its contacts
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			listId		path		string	true	"Broadcast list ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/lists/{listId} [get]
func (h *Handler) getBroadcastList(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	listID, err := uuid.Parse(chi.URLParam(r, "listId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid list id")
		return
	}
	list, contacts, err := h.svc.GetBroadcastListWithContacts(r.Context(), studioID, listID)
	if err != nil {
		if err == ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "list not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"list": list, "contacts": contacts})
}

// deleteBroadcastList godoc
//
//	@Summary		Delete a broadcast list
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID"
//	@Param			listId		path	string	true	"Broadcast list ID"
//	@Success		204
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/lists/{listId} [delete]
func (h *Handler) deleteBroadcastList(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	listID, err := uuid.Parse(chi.URLParam(r, "listId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid list id")
		return
	}
	if err := h.svc.DeleteBroadcastList(r.Context(), studioID, listID); err != nil {
		if err == ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "list not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.NoContent(w)
}

type createBroadcastCampaignReq struct {
	BroadcastListID uuid.UUID        `json:"broadcastListId"`
	Channel         BroadcastChannel `json:"channel"` // "whatsapp" (default) or "email"
	Subject         string           `json:"subject"` // required when channel is "email"
	Body            string           `json:"body"`
	Attachments     []Attachment     `json:"attachments"`
	ScheduledFor    *time.Time       `json:"scheduledFor"` // nil/omitted means "as soon as today's limit allows"
}

// createBroadcastCampaign godoc
//
//	@Summary		Schedule a WhatsApp broadcast to a contact list
//	@Description	Schedules body/attachments to send to every contact in broadcastListId. If the list is larger than the studio's daily WhatsApp send limit, the remainder sends automatically on subsequent days — see internal/messaging's BroadcastWorker.
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string						true	"Studio ID"
//	@Param			body		body		createBroadcastCampaignReq	true	"Campaign details"
//	@Success		201			{object}	BroadcastCampaign
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/campaigns [post]
func (h *Handler) createBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req createBroadcastCampaignReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	in := CreateBroadcastCampaignInput{
		BroadcastListID: req.BroadcastListID,
		Channel:         req.Channel,
		Subject:         req.Subject,
		Body:            req.Body,
		Attachments:     req.Attachments,
	}
	if req.ScheduledFor != nil {
		in.ScheduledFor = *req.ScheduledFor
	}
	campaign, err := h.svc.CreateBroadcastCampaign(r.Context(), studioID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, campaign)
}

// listBroadcastCampaigns godoc
//
//	@Summary		List WhatsApp broadcast campaigns
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]interface{}
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/campaigns [get]
func (h *Handler) listBroadcastCampaigns(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	campaigns, err := h.svc.ListBroadcastCampaigns(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"campaigns": campaigns})
}

// cancelBroadcastCampaign godoc
//
//	@Summary		Cancel a scheduled or in-progress WhatsApp broadcast
//	@Description	Stops enqueueing any remaining pending recipients. Recipients already enqueued into outbound_jobs are not recalled.
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID"
//	@Param			campaignId	path	string	true	"Campaign ID"
//	@Success		204
//	@Failure		404	{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/campaigns/{campaignId} [delete]
func (h *Handler) cancelBroadcastCampaign(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	campaignID, err := uuid.Parse(chi.URLParam(r, "campaignId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid campaign id")
		return
	}
	if err := h.svc.CancelBroadcastCampaign(r.Context(), studioID, campaignID); err != nil {
		if err == ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "campaign not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.NoContent(w)
}

// listBroadcastCampaignRecipients godoc
//
//	@Summary		List a broadcast's recipients with real send status
//	@Description	Per-recipient delivery state (pending/sent/failed/dead), joined through to the actual outbound_jobs record once enqueued — sentAt is set only once Meta/WhatsApp confirms delivery, not when it's merely queued.
//	@Tags			Messaging - Broadcasts
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			campaignId	path		string	true	"Campaign ID"
//	@Success		200			{object}	map[string]interface{}
//	@Router			/api/v1/studios/{studioId}/messaging/broadcasts/campaigns/{campaignId}/recipients [get]
func (h *Handler) listBroadcastCampaignRecipients(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	campaignID, err := uuid.Parse(chi.URLParam(r, "campaignId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid campaign id")
		return
	}
	recipients, err := h.svc.ListBroadcastCampaignRecipients(r.Context(), studioID, campaignID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"recipients": recipients})
}
