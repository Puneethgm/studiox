package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MetaGraphBaseURL is the Meta Graph API root. Override in tests.
var MetaGraphBaseURL = "https://graph.facebook.com"

// MetaWhatsApp talks to Meta's WhatsApp Cloud API directly. One HTTP client,
// many studios — each call carries the studio's per-channel access token.
type MetaWhatsApp struct {
	apiVersion string // e.g. "v25.0"
	httpClient *http.Client
}

func NewMetaWhatsApp(apiVersion string) *MetaWhatsApp {
	if apiVersion == "" {
		apiVersion = "v25.0"
	}
	return &MetaWhatsApp{
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// uploadMediaToMeta reads a local file from disk, uploads it to Meta's
// WhatsApp Media API, and returns the media_id.
// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media#upload-media
func (m *MetaWhatsApp) uploadMediaToMeta(ctx context.Context, accessToken, phoneNumberID, localPath string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open local file %q: %w", localPath, err)
	}
	defer f.Close()

	// Detect MIME type from file extension.
	ext := strings.ToLower(filepath.Ext(localPath))
	mimeType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".mp4":
		mimeType = "video/mp4"
	case ".mov":
		mimeType = "video/quicktime"
	case ".pdf":
		mimeType = "application/pdf"
	case ".doc":
		mimeType = "application/msword"
	case ".docx":
		mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	// Build multipart body required by Meta's media upload endpoint.
	// IMPORTANT: Meta validates the Content-Type of the file part — it must
	// match the actual media type (e.g. image/jpeg). The standard
	// multipart.Writer.CreateFormFile always sets application/octet-stream,
	// so we use CreatePart with a hand-crafted header instead.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// messaging_product field (required by Meta)
	if fw, err2 := mw.CreateFormField("messaging_product"); err2 == nil {
		_, _ = fw.Write([]byte("whatsapp"))
	}

	// File part with correct Content-Type
	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(localPath)),
	}
	partHeader["Content-Type"] = []string{mimeType}

	part, err := mw.CreatePart(partHeader)
	if err != nil {
		return "", fmt.Errorf("create form part: %w", err)
	}
	if _, err = io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}
	mw.Close()

	uploadURL := fmt.Sprintf("%s/%s/%s/media", MetaGraphBaseURL, m.apiVersion, phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("media upload http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("meta media upload: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var ok struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &ok); err != nil || ok.ID == "" {
		return "", fmt.Errorf("meta media upload: unexpected response: %s", string(respBody))
	}
	return ok.ID, nil
}

// SendText: POST /{phone_number_id}/messages.
//
// When an attachment is present:
//   - If the URL is a local /uploads/... path, the file is first uploaded to
//     Meta's Media API to obtain a media_id; the message is then sent using
//     "id": <media_id>. This works even when the Go server has no public IP.
//   - If the URL already starts with https://, it is sent as "link": <url>.
//
// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages
func (m *MetaWhatsApp) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error) {
	// In local dev mode with invalid/empty credentials, mock the send.
	if os.Getenv("API_ENV") == "local" && (accessToken == "" || accessToken == "test") {
		return &SendResult{
			ExternalID: "wamid-test-" + time.Now().Format("20060102150405"),
		}, nil
	}
	if accessToken == "" {
		return nil, ErrInvalidCredentials
	}
	messagesURL := fmt.Sprintf("%s/%s/%s/messages", MetaGraphBaseURL, m.apiVersion, channelExternalID)

	// ── Button detection ─────────────────────────────────────────────────────
	// If body lines look like "1. Option", "2. Option" (2-3 of them) we send
	// as a WhatsApp interactive button message instead of plain text.
	useButtons := false
	var btnBody string
	var buttons []map[string]any

	lines := strings.Split(body, "\n")
	var parsedButtons []string
	var cleanBodyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "1. "):
			parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
		case strings.HasPrefix(trimmed, "2. "):
			parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
		case strings.HasPrefix(trimmed, "3. "):
			parsedButtons = append(parsedButtons, strings.TrimSpace(trimmed[3:]))
		default:
			cleanBodyLines = append(cleanBodyLines, line)
		}
	}

	if len(parsedButtons) >= 1 && len(parsedButtons) <= 3 {
		useButtons = true
		btnBody = strings.TrimSpace(strings.Join(cleanBodyLines, "\n"))
		if btnBody == "" {
			btnBody = "Please choose an option:"
		}
		for idx, btnText := range parsedButtons {
			if len(btnText) > 20 {
				btnText = btnText[:20]
			}
			buttons = append(buttons, map[string]any{
				"type": "reply",
				"reply": map[string]any{
					"id":    fmt.Sprintf("choice_%d", idx+1),
					"title": btnText,
				},
			})
		}
	}

	// ── Payload construction ─────────────────────────────────────────────────
	var payload map[string]any

	if len(attachments) > 0 && attachments[0].URL != "" {
		att := attachments[0]
		mediaType := att.Type
		if mediaType == "" {
			mediaType = "image"
		}

		mediaObj := map[string]any{}

		attURL := att.URL
		if strings.HasPrefix(attURL, "/uploads/") {
			// Local file: upload to Meta first → use id:
			localPath := filepath.Join("uploads", strings.TrimPrefix(attURL, "/uploads/"))
			mediaID, uploadErr := m.uploadMediaToMeta(ctx, accessToken, channelExternalID, localPath)
			if uploadErr != nil {
				// Non-fatal: fall back to link (only works if server is public)
				fmt.Printf("[WARN] Meta media upload failed (%v); falling back to link\n", uploadErr)
				mediaObj["link"] = attURL
			} else {
				mediaObj["id"] = mediaID
			}
		} else {
			// Already a public https:// URL
			mediaObj["link"] = attURL
		}

		if body != "" {
			mediaObj["caption"] = body
		}

		if mediaType == "document" {
			fn := att.Name
			if fn == "" {
				fn = filepath.Base(attURL)
			}
			if fn != "" {
				mediaObj["filename"] = fn
			}
		}

		payload = map[string]any{
			"messaging_product": "whatsapp",
			"recipient_type":    "individual",
			"to":                recipient,
			"type":              mediaType,
			mediaType:           mediaObj,
		}
	} else if useButtons {
		payload = map[string]any{
			"messaging_product": "whatsapp",
			"recipient_type":    "individual",
			"to":                recipient,
			"type":              "interactive",
			"interactive": map[string]any{
				"type": "button",
				"body": map[string]any{
					"text": btnBody,
				},
				"action": map[string]any{
					"buttons": buttons,
				},
			},
		}
	} else {
		payload = map[string]any{
			"messaging_product": "whatsapp",
			"to":                recipient,
			"type":              "text",
			"text":              map[string]any{"body": body, "preview_url": true},
		}
	}

	// ── HTTP POST ─────────────────────────────────────────────────────────────
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		// Meta error envelope: {"error":{"message":"...","type":"...","code":N,...}}
		var errEnv struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errEnv)
		if resp.StatusCode == http.StatusUnauthorized || errEnv.Error.Code == 190 {
			return nil, fmt.Errorf("%w: meta whatsapp auth failed: HTTP %d %s (code=%d, type=%s)",
				ErrInvalidCredentials, resp.StatusCode, errEnv.Error.Message, errEnv.Error.Code, errEnv.Error.Type)
		}
		isLocalDev := os.Getenv("API_ENV") == "local"
		if isLocalDev && (resp.StatusCode == http.StatusForbidden || errEnv.Error.Code == 131005 || errEnv.Error.Code == 131030 || errEnv.Error.Code == 100) {
			fmt.Printf("[Meta API Error Mapped to Mock] HTTP Status %d, Error Body: %s\n", resp.StatusCode, string(respBody))
			return &SendResult{
				ExternalID: "wamid-mock-" + time.Now().Format("20060102150405"),
			}, nil
		}
		if errEnv.Error.Message != "" {
			if resp.StatusCode == http.StatusUnauthorized || errEnv.Error.Code == 190 {
				return nil, fmt.Errorf("%w: meta whatsapp auth failed: HTTP %d %s (code=%d, type=%s)",
					ErrInvalidCredentials, resp.StatusCode, errEnv.Error.Message, errEnv.Error.Code, errEnv.Error.Type)
			}
			return nil, fmt.Errorf("meta whatsapp send: HTTP %d %s (code=%d, type=%s)",
				resp.StatusCode, errEnv.Error.Message, errEnv.Error.Code, errEnv.Error.Type)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: meta whatsapp auth failed: HTTP %d", ErrInvalidCredentials, resp.StatusCode)
		}
		return nil, fmt.Errorf("meta whatsapp send: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Success envelope: {"messaging_product":"whatsapp","contacts":[...],"messages":[{"id":"wamid....","message_status":"accepted"}]}
	var ok struct {
		Messages []struct {
			ID            string `json:"id"`
			MessageStatus string `json:"message_status"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(respBody, &ok); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, string(respBody))
	}
	if len(ok.Messages) == 0 || ok.Messages[0].ID == "" {
		return nil, fmt.Errorf("meta whatsapp send: empty messages array (body=%s)", string(respBody))
	}
	return &SendResult{ExternalID: ok.Messages[0].ID}, nil
}

// ============================================================
// Webhook payload parsing
// ============================================================

// WhatsAppWebhookPayload mirrors the relevant subset of Meta's payload shape.
// We unmarshal the whole thing and walk it; messages and statuses arrive
// inside `entry[].changes[].value`.
//
// Reference:
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples
type WhatsAppWebhookPayload struct {
	Object string                 `json:"object"`
	Entry  []WhatsAppWebhookEntry `json:"entry"`
}

type WhatsAppWebhookEntry struct {
	ID      string                  `json:"id"` // WABA id
	Changes []WhatsAppWebhookChange `json:"changes"`
}

type WhatsAppWebhookChange struct {
	Field string               `json:"field"`
	Value WhatsAppWebhookValue `json:"value"`
}

type WhatsAppWebhookValue struct {
	MessagingProduct string                   `json:"messaging_product"`
	Metadata         WhatsAppWebhookMetadata  `json:"metadata"`
	Contacts         []WhatsAppWebhookContact `json:"contacts,omitempty"`
	Messages         []WhatsAppWebhookMessage `json:"messages,omitempty"`
	Statuses         []WhatsAppWebhookStatus  `json:"statuses,omitempty"`
}

type WhatsAppWebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"` // <-- our channel external_id
}

type WhatsAppWebhookContact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WAID string `json:"wa_id"` // contact phone in international format
}

type WhatsAppWebhookMessage struct {
	From      string `json:"from"`      // contact phone (no '+')
	ID        string `json:"id"`        // wamid....
	Timestamp string `json:"timestamp"` // unix seconds, as string
	Type      string `json:"type"`      // text | image | audio | video | document | reaction | ...
	Text      *struct {
		Body string `json:"body"`
	} `json:"text,omitempty"`
	Image       *WhatsAppWebhookMedia       `json:"image,omitempty"`
	Video       *WhatsAppWebhookMedia       `json:"video,omitempty"`
	Audio       *WhatsAppWebhookMedia       `json:"audio,omitempty"`
	Document    *WhatsAppWebhookMedia       `json:"document,omitempty"`
	Interactive *WhatsAppWebhookInteractive `json:"interactive,omitempty"`
	Button      *WhatsAppWebhookButton      `json:"button,omitempty"`
	Context     *struct {
		ID   string `json:"id"`
		From string `json:"from"`
	} `json:"context,omitempty"`
}

type WhatsAppWebhookInteractive struct {
	Type        string                           `json:"type"` // button_reply, list_reply
	ButtonReply *WhatsAppWebhookInteractiveReply `json:"button_reply,omitempty"`
	ListReply   *WhatsAppWebhookInteractiveReply `json:"list_reply,omitempty"`
}

type WhatsAppWebhookInteractiveReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type WhatsAppWebhookButton struct {
	Payload string `json:"payload"`
	Text    string `json:"text"`
}

type WhatsAppWebhookMedia struct {
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	ID       string `json:"id"`
	Caption  string `json:"caption,omitempty"`
}

type WhatsAppWebhookStatus struct {
	ID          string `json:"id"`     // outbound wamid we sent earlier
	Status      string `json:"status"` // sent | delivered | read | failed
	Timestamp   string `json:"timestamp"`
	RecipientID string `json:"recipient_id"`
}

// ParseTimestamp converts Meta's string-encoded unix seconds into a time.
func ParseTimestamp(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	// Meta uses unix-seconds as a string. strconv-free parse to keep deps tight.
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return time.Now().UTC()
		}
		n = n*10 + int64(c-'0')
	}
	return time.Unix(n, 0).UTC()
}
