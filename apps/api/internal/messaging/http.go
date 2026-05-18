package messaging

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

// AdminRoutes are mounted under /api/v1/studios/{studioId}/messaging.
// Studio scoping comes from the surrounding middleware (resolveStudioID +
// RequireActiveStudio); we just trust the path's studioId here.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/channels", h.listChannels)
	r.Post("/channels/whatsapp", h.connectWhatsApp)
	r.Post("/channels/instagram", h.connectInstagram)
	r.Post("/channels/messenger", h.connectMessenger)
	r.Delete("/channels/{id}", h.disconnectChannel)

	r.Get("/conversations", h.listConversations)
	r.Post("/conversations", h.createConversation)
	r.Get("/conversations/{id}", h.getConversation)
	r.Get("/conversations/{id}/messages", h.listMessages)
	r.Post("/conversations/{id}/messages", h.sendMessage)
	r.Post("/conversations/{id}/read", h.markRead)

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
	Body string `json:"body"`
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
	id, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_studio_id", "invalid studio id")
		return uuid.Nil, false
	}
	return id, true
}
