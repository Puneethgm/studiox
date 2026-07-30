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

// waWebBackfillTrigger kicks off a one-time chat-history import for a
// QR-linked session. Fire-and-forget from the admin's perspective — progress
// is polled via waWebBackfillStatus.
func (h *Handler) waWebBackfillTrigger(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	if err := h.svc.repo.SetWAWebBackfillStatus(r.Context(), studioID, "running", 0); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	proxyToWAWeb(w, r.Context(), http.MethodPost,
		fmt.Sprintf("%s/sessions/%s/backfill", waWebServiceURL(), studioID))
}

func (h *Handler) waWebBackfillStatus(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	status, count, err := h.svc.repo.GetWAWebBackfillStatus(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": status, "messageCount": count})
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
		FromMe    bool   `json:"fromMe"`
		PushName  string `json:"pushName"`
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
	if err := h.svc.HandleInboundWAWeb(r.Context(), studioID, p.From, p.Text, p.MessageID, p.FromMe, sentAt, p.PushName); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// waWebContactName records a display name for a WhatsApp contact that has no
// message history to carry it in via waWebBackfill (a saved contact with no
// chat yet, or a chat whose only messages were skipped as non-text) — see
// sessions.js's messaging-history.set handler and Service.HandleWAWebContactName.
func (h *Handler) waWebContactName(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID    string `json:"studioId"`
		JID         string `json:"jid"`
		DisplayName string `json:"displayName"`
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
	if err := h.svc.HandleWAWebContactName(r.Context(), studioID, p.JID, p.DisplayName); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// waWebBackfill receives a batch of historical messages for one chat, imported
// by the wa-web Node service after a QR-linked session connects. Called
// repeatedly, once per chat (or in chat-sized pages), not all-at-once.
func (h *Handler) waWebBackfill(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID    string `json:"studioId"`
		DisplayName string `json:"displayName"`
		Messages    []struct {
			From      string `json:"from"`
			Text      string `json:"text"`
			MessageID string `json:"messageId"`
			Timestamp int64  `json:"timestamp"`
			FromMe    bool   `json:"fromMe"`
		} `json:"messages"`
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
	msgs := make([]BackfillMessage, 0, len(p.Messages))
	for _, m := range p.Messages {
		if m.Text == "" {
			continue // skip media/empty messages — text-only for now
		}
		msgs = append(msgs, BackfillMessage{
			From:      m.From,
			Text:      m.Text,
			MessageID: m.MessageID,
			Timestamp: m.Timestamp,
			FromMe:    m.FromMe,
		})
	}
	imported, err := h.svc.HandleInboundWAWebBackfill(r.Context(), studioID, msgs, p.DisplayName)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "imported": imported})
}

// waWebBackfillRunning lets the Node service mark history import as started
// the moment a session connects — Baileys pushes chat history automatically
// post-pairing, so import can begin without an admin ever clicking the
// "Import chat history" button. Without this, the admin UI would sit on
// "none" (no button pressed yet) while an import is silently already
// underway in the background.
func (h *Handler) waWebBackfillRunning(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.repo.SetWAWebBackfillStatus(r.Context(), studioID, "running", 0); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// waWebBackfillDone lets the Node service report that it has finished walking
// every chat, so the admin UI can stop showing "running".
func (h *Handler) waWebBackfillDone(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID     string `json:"studioId"`
		MessageCount int    `json:"messageCount"`
		Failed       bool   `json:"failed"`
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
	status := "done"
	if p.Failed {
		status = "failed"
	}
	if err := h.svc.repo.SetWAWebBackfillStatus(r.Context(), studioID, status, p.MessageCount); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	// Kick off post-backfill AI summarization of imported conversations, but
	// only on a genuine success — a failed/partial import shouldn't spend LLM
	// calls summarizing incomplete history.
	if status == "done" {
		if channel, err := h.svc.repo.GetActiveChannelByKind(r.Context(), studioID, KindWhatsAppWeb); err == nil && channel != nil {
			h.bus.Publish(r.Context(), Event{
				Kind:             EvtWAWebBackfillDone,
				StudioID:         studioID,
				ChannelAccountID: &channel.ID,
			})
		}
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
		To        string `json:"to"`
		Text      string `json:"text,omitempty"`
		MediaURL  string `json:"mediaUrl,omitempty"`
		MediaType string `json:"mediaType,omitempty"`
		Caption   string `json:"caption,omitempty"`
	}

	// doSend returns the WhatsApp message ID wa-web assigned to the send.
	// Capturing this matters beyond bookkeeping: when this same message is
	// later echoed back to us as a `fromMe` event on the live socket (see
	// SessionManager's messages.upsert handler), the Go side needs a real ID
	// to dedupe against — otherwise a message we sent through the platform
	// would get inserted a second time as if it were typed directly on the
	// phone.
	doSend := func(p sendPayload) (string, error) {
		data, _ := json.Marshal(p)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-internal-key", waWebInternalKey())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("wa-web send failed %d: %s", resp.StatusCode, body)
		}
		var parsed struct {
			MessageID string `json:"messageId"`
		}
		_ = json.Unmarshal(body, &parsed)
		return parsed.MessageID, nil
	}

	// Send text message (unless there are attachments — in that case text becomes caption on first media)
	if len(attachments) == 0 {
		id, err := doSend(sendPayload{To: recipient, Text: body})
		return &channels.SendResult{ExternalID: id}, err
	}

	// Send each attachment; first one carries the caption (body text). The
	// external ID we report back is the first attachment's message —
	// InsertMessage only records one row per outbound job either way.
	var firstID string
	for i, att := range attachments {
		caption := ""
		if i == 0 {
			caption = body
		}
		id, err := doSend(sendPayload{
			To:        recipient,
			MediaURL:  att.URL,
			MediaType: string(att.Type),
			Caption:   caption,
		})
		if err != nil {
			return nil, err
		}
		if i == 0 {
			firstID = id
		}
	}
	return &channels.SendResult{ExternalID: firstID}, nil
}
