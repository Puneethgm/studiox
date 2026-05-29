package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dghubble/oauth1"
)

type XSender struct{}

func NewXSender() *XSender {
	return &XSender{}
}

func (s *XSender) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error) {
	// Parse the access token JSON which contains all 4 OAuth 1.0a secrets.
	var keys struct {
		ConsumerKey       string `json:"consumer_key"`
		ConsumerSecret    string `json:"consumer_secret"`
		AccessToken       string `json:"access_token"`
		AccessTokenSecret string `json:"access_token_secret"`
	}
	if err := json.Unmarshal([]byte(accessToken), &keys); err != nil {
		return nil, fmt.Errorf("invalid x channel credentials: %w", err)
	}

	config := oauth1.NewConfig(keys.ConsumerKey, keys.ConsumerSecret)
	token := oauth1.NewToken(keys.AccessToken, keys.AccessTokenSecret)
	httpClient := config.Client(ctx, token)

	endpoint := "https://api.twitter.com/1.1/direct_messages/events/new.json"

	payload := map[string]interface{}{
		"event": map[string]interface{}{
			"type": "message_create",
			"message_create": map[string]interface{}{
				"target": map[string]string{
					"recipient_id": recipient,
				},
				"message_data": map[string]string{
					"text": body,
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("x api req error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("x api error: %d - %v", resp.StatusCode, errResp)
	}

	var successResp struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&successResp); err != nil {
		return nil, fmt.Errorf("x api decode error: %w", err)
	}

	return &SendResult{ExternalID: successResp.Event.ID}, nil
}
