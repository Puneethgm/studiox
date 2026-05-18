package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MetaGraphBaseURL is the Meta Graph API root. Override in tests.
var MetaGraphBaseURL = "https://graph.facebook.com"

// MetaWhatsApp talks to Meta's WhatsApp Cloud API directly. One HTTP client,
// many studios — each call carries the studio's per-channel access token.
type MetaWhatsApp struct {
	apiVersion string // e.g. "v21.0"
	httpClient *http.Client
}

func NewMetaWhatsApp(apiVersion string) *MetaWhatsApp {
	if apiVersion == "" {
		apiVersion = "v21.0"
	}
	return &MetaWhatsApp{
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// SendText: POST /{phone_number_id}/messages with a "text" payload.
// https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages
func (m *MetaWhatsApp) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string) (*SendResult, error) {
	// In local dev mode with invalid/empty credentials, mock the send.
	if os.Getenv("API_ENV") == "local" && (accessToken == "" || accessToken == "test") {
		return &SendResult{
			ExternalID: "wamid-test-" + time.Now().Format("20060102150405"),
		}, nil
	}
	if accessToken == "" {
		return nil, ErrInvalidCredentials
	}
	url := fmt.Sprintf("%s/%s/%s/messages", MetaGraphBaseURL, m.apiVersion, channelExternalID)

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                recipient,
		"type":              "text",
		"text":              map[string]any{"body": body, "preview_url": true},
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
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
		// In local dev mode, mock credential errors so testing doesn't require valid Meta credentials.
		isLocalDev := os.Getenv("API_ENV") == "local"
		if isLocalDev && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || errEnv.Error.Code == 190 || errEnv.Error.Code == 131005) {
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
	Image    *WhatsAppWebhookMedia `json:"image,omitempty"`
	Video    *WhatsAppWebhookMedia `json:"video,omitempty"`
	Audio    *WhatsAppWebhookMedia `json:"audio,omitempty"`
	Document *WhatsAppWebhookMedia `json:"document,omitempty"`
	Context  *struct {
		ID   string `json:"id"`
		From string `json:"from"`
	} `json:"context,omitempty"`
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
