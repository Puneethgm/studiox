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

// MetaMessenger talks to Meta's Messenger/Instagram Graph API.
type MetaMessenger struct {
	apiVersion string
	httpClient *http.Client
}

func NewMetaMessenger(apiVersion string) *MetaMessenger {
	if apiVersion == "" {
		apiVersion = "v21.0"
	}
	return &MetaMessenger{
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// SendText: POST /{page_id}/messages
// https://developers.facebook.com/docs/messenger-platform/reference/send-api
func (m *MetaMessenger) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string) (*SendResult, error) {
	if os.Getenv("API_ENV") == "local" && (accessToken == "" || accessToken == "test") {
		return &SendResult{
			ExternalID: "mid.test-" + time.Now().Format("20060102150405"),
		}, nil
	}
	if accessToken == "" {
		return nil, ErrInvalidCredentials
	}

	url := fmt.Sprintf("%s/%s/%s/messages", MetaGraphBaseURL, m.apiVersion, channelExternalID)

	payload := map[string]any{
		"recipient": map[string]string{"id": recipient},
		"message":   map[string]string{"text": body},
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
		var errEnv struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &errEnv)
		if errEnv.Error.Code == 190 {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("meta messenger send: HTTP %d: %s", resp.StatusCode, errEnv.Error.Message)
	}

	var ok struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(respBody, &ok); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &SendResult{ExternalID: ok.MessageID}, nil
}
