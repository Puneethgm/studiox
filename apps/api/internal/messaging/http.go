package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	svc *Service
	bus Bus
}

func NewHandler(svc *Service, bus Bus) *Handler {
	return &Handler{svc: svc, bus: bus}
}

func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/links/{id}", h.redirectTriggerLink)
}

// AdminRoutes are mounted under /api/v1/studios/{studioId}/messaging.
// Studio scoping comes from the surrounding middleware (resolveStudioID +
// RequireActiveStudio); we just trust the path's studioId here.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/channels", h.listChannels)
	r.Post("/channels/whatsapp", h.connectWhatsApp)
	r.Post("/channels/instagram", h.connectInstagram)
	r.Post("/channels/messenger", h.connectMessenger)
	r.Delete("/channels/{id}", h.disconnectChannel)
	r.Put("/channels/{id}", h.updateChannel)

	r.Get("/conversations", h.listConversations)
	r.Post("/conversations", h.createConversation)
	r.Get("/conversations/{id}", h.getConversation)
	r.Get("/conversations/{id}/messages", h.listMessages)
	r.Post("/conversations/{id}/messages", h.sendMessage)
	r.Post("/conversations/{id}/read", h.markRead)

	// Templates
	r.Get("/templates", h.listTemplates)
	r.Post("/templates", h.createTemplate)
	r.Put("/templates/{id}", h.updateTemplate)
	r.Delete("/templates/{id}", h.deleteTemplate)

	// Trigger Links
	r.Get("/trigger-links", h.listTriggerLinks)
	r.Post("/trigger-links", h.createTriggerLink)
	r.Put("/trigger-links/{id}", h.updateTriggerLink)
	r.Delete("/trigger-links/{id}", h.deleteTriggerLink)

	// Jobs (Automated / Manual Actions)
	r.Get("/jobs", h.listPendingJobs)
	r.Post("/jobs", h.createJob)
	r.Put("/jobs/{id}", h.updateJob)
	r.Post("/jobs/{id}/trigger", h.triggerJobNow)
	r.Delete("/jobs/{id}", h.deleteJob)

	// AI Assistant
	r.Post("/ai/generate", h.aiGenerateTemplate)

	// File Upload (images / videos / docs for compose area)
	r.Post("/upload", h.uploadMedia)

	r.Get("/stream", h.stream) // SSE — live updates for the inbox UI
}

// ============================================================
// channels
// ============================================================

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListChannels(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"channels": list})
}

type connectMetaReq struct {
	ExternalID    string `json:"externalId"`    // ID or phone
	ParentID      string `json:"parentId"`      // WABA ID or App ID
	DisplayHandle string `json:"displayHandle"` // handle or name
	AccessToken   string `json:"accessToken"`
}

func (h *Handler) connectWhatsApp(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		WABAID        string `json:"wabaId"`
		PhoneNumberID string `json:"phoneNumberId"`
		DisplayPhone  string `json:"displayPhone"`
		AccessToken   string `json:"accessToken"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ch, err := h.svc.ConnectMetaChannel(r.Context(), studioID, ConnectMetaInput{
		Kind:          KindWhatsAppMeta,
		ExternalID:    req.PhoneNumberID,
		ParentID:      req.WABAID,
		DisplayHandle: req.DisplayPhone,
		AccessToken:   req.AccessToken,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, ch)
}

func (h *Handler) connectInstagram(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req connectMetaReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ch, err := h.svc.ConnectMetaChannel(r.Context(), studioID, ConnectMetaInput{
		Kind:          KindInstagramMeta,
		ExternalID:    req.ExternalID,
		ParentID:      req.ParentID,
		DisplayHandle: req.DisplayHandle,
		AccessToken:   req.AccessToken,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, ch)
}

func (h *Handler) connectMessenger(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req connectMetaReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ch, err := h.svc.ConnectMetaChannel(r.Context(), studioID, ConnectMetaInput{
		Kind:          KindMessengerMeta,
		ExternalID:    req.ExternalID,
		ParentID:      req.ParentID,
		DisplayHandle: req.DisplayHandle,
		AccessToken:   req.AccessToken,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, ch)
}

func (h *Handler) disconnectChannel(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.DisconnectChannel(r.Context(), studioID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req struct {
		ExternalID    string `json:"externalId"`
		ParentID      string `json:"parentId"`
		DisplayHandle string `json:"displayHandle"`
		AccessToken   string `json:"accessToken"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ch, err := h.svc.UpdateChannel(r.Context(), studioID, id, req.ExternalID, req.ParentID, req.DisplayHandle, req.AccessToken)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "channel not found")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, ch)
}

// ============================================================
// conversations + messages
// ============================================================

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	f := ListConversationsFilter{}
	if v := q.Get("status"); v != "" {
		s := ConvStatus(v)
		f.Status = &s
	}
	if v := q.Get("limit"); v != "" {
		n, _ := strconv.Atoi(v)
		f.Limit = n
	}
	if v := q.Get("channelKind"); v != "" {
		k := ChannelKind(v)
		if k.Valid() {
			f.ChannelKind = &k
		}
	}
	if v := q.Get("offset"); v != "" {
		n, _ := strconv.Atoi(v)
		f.Offset = n
	}
	list, total, err := h.svc.ListConversations(r.Context(), studioID, f)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"conversations": list, "total": total})
}

func (h *Handler) getConversation(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	c, err := h.svc.GetConversation(r.Context(), studioID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "conversation not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, c)
}

type createConversationReq struct {
	ChannelKind  ChannelKind `json:"channelKind"`
	ContactValue string      `json:"contactValue"`
	DisplayName  string      `json:"displayName"`
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req createConversationReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	conv, err := h.svc.CreateConversation(r.Context(), studioID, CreateConversationInput{
		ChannelKind:  req.ChannelKind,
		ContactValue: req.ContactValue,
		DisplayName:  req.DisplayName,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusBadRequest, "no_channel", "connect a channel before starting a conversation")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, conv)
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, _ := strconv.Atoi(v)
		if n > 0 {
			limit = n
		}
	}
	msgs, err := h.svc.ListMessages(r.Context(), studioID, id, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

type sendMessageReq struct {
	Body        string       `json:"body"`
	Attachments []Attachment `json:"attachments"`
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req sendMessageReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	c := identity.MustClaims(r.Context())
	jobID, err := h.svc.EnqueueReply(r.Context(), SendInput{
		StudioID:       studioID,
		ConversationID: id,
		UserID:         c.UserID,
		Body:           req.Body,
		Attachments:    req.Attachments,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"jobId": jobID})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.MarkRead(r.Context(), studioID, id); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.NoContent(w)
}

// ============================================================
// SSE — live updates for the inbox UI
// ============================================================

// stream sends an event-stream over chunked HTTP. Each event is a
// JSON-serialised messaging.Event. The browser EventSource API auto-reconnects
// with `Last-Event-ID`, but we don't replay history server-side at L1 — clients
// re-fetch on reconnect.

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	// Initial hello so the client knows the stream is live.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ch, unsub := h.bus.Subscribe(studioID)
	defer unsub()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			// Comment line keeps proxies from killing the connection.
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\n", evt.Kind)
			fmt.Fprintf(w, "data: %s\n\n", evt.JSON())
			flusher.Flush()
		}
	}
}

// ============================================================
// helpers
// ============================================================

// studioIDFromPath extracts the studioId path param. Always present because
// the routes mount under /studios/{studioId}/messaging.
func studioIDFromPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	c := identity.MustClaims(r.Context())
	if c.IsSuper() {
		id, err := uuid.Parse(chi.URLParam(r, "studioId"))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_studio_id", "invalid studio id")
			return uuid.Nil, false
		}
		return id, true
	}
	if c.StudioID == nil {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
		return uuid.Nil, false
	}
	return *c.StudioID, true
}

// ============================================================
// templates handlers
// ============================================================

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListTemplates(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		Name         string       `json:"name"`
		Body         string       `json:"body"`
		ChannelKinds []string     `json:"channelKinds"`
		Attachments  []Attachment `json:"attachments"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	mt, err := h.svc.CreateTemplate(r.Context(), studioID, req.Name, req.Body, req.ChannelKinds, req.Attachments)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, mt)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req struct {
		Name         string       `json:"name"`
		Body         string       `json:"body"`
		ChannelKinds []string     `json:"channelKinds"`
		Attachments  []Attachment `json:"attachments"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	mt, err := h.svc.UpdateTemplate(r.Context(), studioID, id, req.Name, req.Body, req.ChannelKinds, req.Attachments)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, mt)
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.DeleteTemplate(r.Context(), studioID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.NoContent(w)
}

// ============================================================
// trigger links handlers
// ============================================================

func (h *Handler) listTriggerLinks(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListTriggerLinks(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"triggerLinks": list})
}

func (h *Handler) createTriggerLink(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tl, err := h.svc.CreateTriggerLink(r.Context(), studioID, req.Name, req.URL)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, tl)
}

func (h *Handler) updateTriggerLink(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tl, err := h.svc.UpdateTriggerLink(r.Context(), studioID, id, req.Name, req.URL)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "trigger link not found")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, tl)
}

func (h *Handler) deleteTriggerLink(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.DeleteTriggerLink(r.Context(), studioID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "trigger link not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) redirectTriggerLink(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	tl, err := h.svc.GetTriggerLinkByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var leadIDPtr *uuid.UUID
	if qLeadID := r.URL.Query().Get("leadId"); qLeadID != "" {
		if lid, err := uuid.Parse(qLeadID); err == nil {
			leadIDPtr = &lid
		}
	}
	_ = h.svc.RecordTriggerLinkClick(r.Context(), id, leadIDPtr)
	http.Redirect(w, r, tl.URL, http.StatusFound)
}

// ============================================================
// outbound jobs / manual actions handlers
// ============================================================

func (h *Handler) listPendingJobs(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListPendingJobs(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"jobs": list})
}

func (h *Handler) triggerJobNow(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.TriggerJobNow(r.Context(), studioID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "pending job not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) deleteJob(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	if err := h.svc.DeleteJob(r.Context(), studioID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "pending job not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.NoContent(w)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	var req struct {
		ConversationID string       `json:"conversationId"`
		Body           string       `json:"body"`
		ScheduledFor   string       `json:"scheduledFor"`
		Attachments    []Attachment `json:"attachments"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	convID, err := uuid.Parse(req.ConversationID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_conversation_id", "invalid conversation id")
		return
	}
	var sched time.Time
	if req.ScheduledFor != "" {
		sched, err = time.Parse(time.RFC3339, req.ScheduledFor)
		if err != nil {
			sched, err = time.Parse("2006-01-02T15:04", req.ScheduledFor)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_time", "invalid scheduled time format")
				return
			}
		}
	} else {
		sched = time.Now().UTC()
	}

	id, err := h.svc.CreateJob(r.Context(), studioID, convID, req.Body, sched, req.Attachments)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) updateJob(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req struct {
		Body         string       `json:"body"`
		ScheduledFor string       `json:"scheduledFor"`
		Attachments  []Attachment `json:"attachments"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	var sched time.Time
	if req.ScheduledFor != "" {
		sched, err = time.Parse(time.RFC3339, req.ScheduledFor)
		if err != nil {
			sched, err = time.Parse("2006-01-02T15:04", req.ScheduledFor)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_time", "invalid scheduled time format")
				return
			}
		}
	} else {
		sched = time.Now().UTC()
	}

	if err := h.svc.UpdateJob(r.Context(), studioID, id, req.Body, sched, req.Attachments); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "pending job not found")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	httpx.NoContent(w)
}

// ============================================================
// AI assistant handlers
// ============================================================

func callGeminiAPI(ctx context.Context, apiKey string, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		return "", err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini API")
	}

	return res.Candidates[0].Content.Parts[0].Text, nil
}

func (h *Handler) aiGenerateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}

	var apiKey string
	err := h.svc.repo.Pool().QueryRow(r.Context(), `
		SELECT gemini_api_key FROM studios WHERE id = $1
	`, studioID).Scan(&apiKey)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load studio config")
		return
	}

	if apiKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_api_key", "Please configure your Gemini API Key in the Studio Settings to write templates with AI.")
		return
	}

	systemInstruction := `Generate a professional, friendly customer message template for a fitness/yoga studio.
Important:
1. The message must NOT contain any salutation or greeting (e.g. do not start with "Hi" or "Dear" or "Hello").
2. The message must NOT contain any sign-off or signature (e.g. do not end with "Best" or "Regards" or "Studio Team").
3. Make it brief, conversational, and direct.
4. If the instruction references a plan, campaign, or link, write the copy naturally.

Generate the message content based on this instruction: ` + req.Prompt

	generatedText, err := callGeminiAPI(r.Context(), apiKey, systemInstruction)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "gemini_error", fmt.Sprintf("AI Generation failed: %v", err))
		return
	}

	body := fmt.Sprintf("Hi {{contact.first_name}},\n\n%s\n\nBest,\n{{studio.name}} Team", strings.TrimSpace(generatedText))
	httpx.JSON(w, http.StatusOK, map[string]string{"body": body})
}

// uploadMedia accepts a multipart/form-data upload (field "file"), saves it
// to apps/api/uploads/<uuid>.<ext>, and returns {"url":"/uploads/<file>"}.
// The caller then includes that URL as an attachment when sending the message.
func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	const maxSize = 20 << 20 // 20 MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file too large or bad multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "missing file field")
		return
	}
	defer file.Close()

	// Derive extension from Content-Type header or filename.
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ct := header.Header.Get("Content-Type")
		exts, _ := mime.ExtensionsByType(ct)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}

	// Only allow safe media types.
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".mp4": true, ".mov": true, ".pdf": true,
		".doc": true, ".docx": true, ".txt": true, ".csv": true,
		".json": true, ".md": true,
	}
	if !allowed[strings.ToLower(ext)] {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "unsupported file type")
		return
	}

	// Ensure uploads directory exists (relative to server CWD = apps/api).
	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not create uploads directory")
		return
	}

	filename := uuid.New().String() + ext
	dst, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not save file")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not write file")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"url":      "/uploads/" + filename,
		"filename": header.Filename,
	})
}
