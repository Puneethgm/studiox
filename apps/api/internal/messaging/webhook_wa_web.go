package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/httpx"
)

func waWebServiceURL() string {
	if u := os.Getenv("WA_WEB_SERVICE_URL"); u != "" {
		return u
	}
	return "http://wa-web:3100"
}

func waWebInternalKey() string {
	if k := os.Getenv("INTERNAL_API_KEY"); k != "" {
		return k
	}
	return "changeme"
}

// ============================================================
// Admin routes — proxy to wa-web Node service
// ============================================================

func (h *Handler) waWebQR(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	proxyToWAWeb(w, r.Context(), http.MethodGet,
		fmt.Sprintf("%s/sessions/%s/qr", waWebServiceURL(), studioID))
}

func (h *Handler) waWebDisconnect(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	_ = h.svc.repo.DisconnectWAWebChannel(r.Context(), studioID)
	proxyToWAWeb(w, r.Context(), http.MethodPost,
		fmt.Sprintf("%s/sessions/%s/disconnect", waWebServiceURL(), studioID))
}

func (h *Handler) waWebStatus(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	proxyToWAWeb(w, r.Context(), http.MethodGet,
		fmt.Sprintf("%s/sessions/%s/status", waWebServiceURL(), studioID))
}

func proxyToWAWeb(w http.ResponseWriter, ctx context.Context, method, url string) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	req.Header.Set("x-internal-key", waWebInternalKey())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"none","error":"wa-web service unavailable"}`)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// ============================================================
// Internal routes — called by wa-web Node service
// ============================================================

func (h *Handler) waWebConnected(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID string `json:"studioId"`
		Phone    string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	studioID, err := uuid.Parse(p.StudioID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studioId")
		return
	}
	if err := h.svc.repo.UpsertWAWebChannel(r.Context(), studioID, p.Phone); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) waWebDisconnected(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID string `json:"studioId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	studioID, err := uuid.Parse(p.StudioID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studioId")
		return
	}
	_ = h.svc.repo.DisconnectWAWebChannel(r.Context(), studioID)
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) waWebInbound(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID  string `json:"studioId"`
		From      string `json:"from"`
		Text      string `json:"text"`
		MessageID string `json:"messageId"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	studioID, err := uuid.Parse(p.StudioID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studioId")
		return
	}
	sentAt := time.Now().UTC()
	if p.Timestamp > 0 {
		sentAt = time.Unix(p.Timestamp, 0).UTC()
	}
	if err := h.svc.HandleInboundWAWeb(r.Context(), studioID, p.From, p.Text, p.MessageID, sentAt); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) waWebStudios(w http.ResponseWriter, r *http.Request) {
	ids, err := h.svc.repo.ListWAWebStudioIDs(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	studioIds := make([]string, len(ids))
	for i, id := range ids {
		studioIds[i] = id.String()
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"studioIds": studioIds})
}

// waWebSender implements channels.Sender by calling the wa-web Node service.
type waWebSender struct {
	studioID uuid.UUID
}

func (s *waWebSender) SendText(ctx context.Context, _, _, recipient, body string, attachments []channels.Attachment) (*channels.SendResult, error) {
	baseURL := waWebServiceURL()
	sendURL := fmt.Sprintf("%s/sessions/%s/send", baseURL, s.studioID)

	type sendPayload struct {
		To          string `json:"to"`
		Text        string `json:"text,omitempty"`
		MediaURL    string `json:"mediaUrl,omitempty"`
		MediaType   string `json:"mediaType,omitempty"`
		Caption     string `json:"caption,omitempty"`
	}

	doSend := func(p sendPayload) error {
		data, _ := json.Marshal(p)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-internal-key", waWebInternalKey())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("wa-web send failed %d: %s", resp.StatusCode, b)
		}
		return nil
	}

	// Send text message (unless there are attachments — in that case text becomes caption on first media)
	if len(attachments) == 0 {
		return &channels.SendResult{}, doSend(sendPayload{To: recipient, Text: body})
	}

	// Send each attachment; first one carries the caption (body text)
	for i, att := range attachments {
		caption := ""
		if i == 0 {
			caption = body
		}
		if err := doSend(sendPayload{
			To:        recipient,
			MediaURL:  att.URL,
			MediaType: string(att.Type),
			Caption:   caption,
		}); err != nil {
			return nil, err
		}
	}
	return &channels.SendResult{}, nil
}
