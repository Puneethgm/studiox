package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/httpx"
)

// tg-web is the Node/teleproto sibling of wa-web (webhook_wa_web.go) —
// same proxy/internal-receiver shape, same x-internal-key auth. See that
// file's header comment; this one only calls out where Telegram differs.

func tgWebServiceURL() string {
	if u := os.Getenv("TG_WEB_SERVICE_URL"); u != "" {
		return u
	}
	return "http://tg-web:3101"
}

func tgWebInternalKey() string {
	// Shares the same internal key as wa-web — both are internal-network-only
	// sidecars authenticated by the same shared secret.
	return waWebInternalKey()
}

// ============================================================
// Admin routes — proxy to tg-web Node service
// ============================================================

func (h *Handler) tgWebQR(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	proxyToTGWeb(w, r.Context(), http.MethodGet,
		fmt.Sprintf("%s/sessions/%s/qr", tgWebServiceURL(), studioID))
}

func (h *Handler) tgWebPassword(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		fmt.Sprintf("%s/sessions/%s/password", tgWebServiceURL(), studioID), bytes.NewReader(body))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-internal-key", tgWebInternalKey())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "tg_web_unavailable", "tg-web service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

func (h *Handler) tgWebDisconnect(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	_ = h.svc.repo.DisconnectTGWebChannel(r.Context(), studioID)
	proxyToTGWeb(w, r.Context(), http.MethodPost,
		fmt.Sprintf("%s/sessions/%s/disconnect", tgWebServiceURL(), studioID))
}

func (h *Handler) tgWebStatus(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	proxyToTGWeb(w, r.Context(), http.MethodGet,
		fmt.Sprintf("%s/sessions/%s/status", tgWebServiceURL(), studioID))
}

func (h *Handler) tgWebBackfillTrigger(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	if err := h.svc.repo.SetTGWebBackfillStatus(r.Context(), studioID, "running", 0); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	proxyToTGWeb(w, r.Context(), http.MethodPost,
		fmt.Sprintf("%s/sessions/%s/backfill", tgWebServiceURL(), studioID))
}

func (h *Handler) tgWebBackfillStatus(w http.ResponseWriter, r *http.Request) {
	studioID, ok := studioIDFromPath(w, r)
	if !ok {
		return
	}
	status, count, err := h.svc.repo.GetTGWebBackfillStatus(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": status, "messageCount": count})
}

func proxyToTGWeb(w http.ResponseWriter, ctx context.Context, method, url string) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	req.Header.Set("x-internal-key", tgWebInternalKey())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"none","error":"tg-web service unavailable"}`)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func notifyTGWebDisconnect(ctx context.Context, studioID uuid.UUID) {
	url := fmt.Sprintf("%s/sessions/%s/disconnect", tgWebServiceURL(), studioID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("x-internal-key", tgWebInternalKey())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// ============================================================
// Internal routes — called by tg-web Node service
// ============================================================

func (h *Handler) tgWebConnected(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID      string `json:"studioId"`
		Phone         string `json:"phone"`
		Username      string `json:"username"`
		SessionString string `json:"sessionString"`
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
	if err := h.svc.repo.UpsertTGWebChannel(r.Context(), studioID, p.Phone, p.Username, p.SessionString); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) tgWebInbound(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID       string `json:"studioId"`
		ChatID         string `json:"chatId"`
		Text           string `json:"text"`
		MessageID      string `json:"messageId"`
		Timestamp      int64  `json:"timestamp"`
		FromMe         bool   `json:"fromMe"`
		DisplayName    string `json:"displayName"`
		AttachmentURL  string `json:"attachmentUrl"`
		AttachmentType string `json:"attachmentType"`
		AttachmentMime string `json:"attachmentMime"`
		AttachmentName string `json:"attachmentName"`
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
	var atts []Attachment
	if p.AttachmentURL != "" {
		atts = append(atts, Attachment{Type: p.AttachmentType, URL: p.AttachmentURL, Mime: p.AttachmentMime, Name: p.AttachmentName})
	}
	if err := h.svc.HandleInboundTGWeb(r.Context(), studioID, p.ChatID, p.Text, p.MessageID, p.FromMe, p.DisplayName, atts, sentAt); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// tgWebMedia receives raw media bytes tg-web already downloaded from
// Telegram (via GramJS's downloadMedia) and saves them under ./uploads,
// same convention every other channel's inbound media uses. Unlike the
// bot's TelegramDownloadFile (which the Go API can call directly against
// Telegram's HTTP file API), an MTProto session's media can only be
// fetched through the GramJS client that owns it — tg-web is the only
// thing that can do that download, so it pushes the resulting bytes here
// rather than the Go side pulling them.
func (h *Handler) tgWebMedia(w http.ResponseWriter, r *http.Request) {
	const maxSize = 20 << 20 // 20 MB, matches uploadMedia's limit
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

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(r.FormValue("mimeType")); len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		ext = ".bin"
	}

	if err := os.MkdirAll("uploads", 0o755); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not create uploads directory")
		return
	}
	filename := "telegram-" + uuid.New().String() + ext
	dst, err := os.Create(filepath.Join("uploads", filename))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not save file")
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "could not write file")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"url": "/uploads/" + filename, "name": filename})
}

func (h *Handler) tgWebBackfill(w http.ResponseWriter, r *http.Request) {
	var p struct {
		StudioID    string `json:"studioId"`
		DisplayName string `json:"displayName"`
		Messages    []struct {
			ChatID    string `json:"chatId"`
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
			continue
		}
		msgs = append(msgs, BackfillMessage{
			From:      m.ChatID,
			Text:      m.Text,
			MessageID: m.MessageID,
			Timestamp: m.Timestamp,
			FromMe:    m.FromMe,
		})
	}
	imported, err := h.svc.HandleInboundTGWebBackfill(r.Context(), studioID, msgs, p.DisplayName)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "imported": imported})
}

func (h *Handler) tgWebBackfillRunning(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.repo.SetTGWebBackfillStatus(r.Context(), studioID, "running", 0); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) tgWebBackfillDone(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.repo.SetTGWebBackfillStatus(r.Context(), studioID, status, p.MessageCount); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	if status == "done" {
		if channel, err := h.svc.repo.GetActiveChannelByKind(r.Context(), studioID, KindTelegramMTProto); err == nil && channel != nil {
			h.bus.Publish(r.Context(), Event{
				Kind:             EvtWAWebBackfillDone, // shared "a QR-linked backfill finished" event — AI summarization doesn't care which channel
				StudioID:         studioID,
				ChannelAccountID: &channel.ID,
			})
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// tgWebSessions feeds tg-web's startup rehydration (see prewarmAll in
// apps/tg-web/src/index.js) — unlike wa-web's ListWAWebStudioIDs (just IDs;
// Baileys' real auth lives on a Docker volume), this must return the actual
// decrypted session strings since the DB is tg-web's only persistence.
func (h *Handler) tgWebSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.svc.repo.ListTGWebSessions(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]map[string]string, len(sessions))
	for i, s := range sessions {
		out[i] = map[string]string{"studioId": s.StudioID.String(), "sessionString": s.SessionString}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": out})
}

// tgWebSender implements channels.Sender by calling the tg-web Node service.
type tgWebSender struct {
	studioID uuid.UUID
}

func (s *tgWebSender) SendText(ctx context.Context, _, _, recipient, body string, attachments []channels.Attachment) (*channels.SendResult, error) {
	sendURL := fmt.Sprintf("%s/sessions/%s/send", tgWebServiceURL(), s.studioID)

	type sendPayload struct {
		To        string `json:"to"`
		Text      string `json:"text,omitempty"`
		MediaURL  string `json:"mediaUrl,omitempty"`
		MediaType string `json:"mediaType,omitempty"`
		Caption   string `json:"caption,omitempty"`
	}

	doSend := func(p sendPayload) (string, error) {
		data, _ := json.Marshal(p)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-internal-key", tgWebInternalKey())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("tg-web send failed %d: %s", resp.StatusCode, body)
		}
		var parsed struct {
			MessageID string `json:"messageId"`
		}
		_ = json.Unmarshal(body, &parsed)
		return parsed.MessageID, nil
	}

	if len(attachments) == 0 {
		id, err := doSend(sendPayload{To: recipient, Text: body})
		return &channels.SendResult{ExternalID: id}, err
	}

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
