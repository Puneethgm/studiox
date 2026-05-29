package channels

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TwilioSMS struct {
	httpClient *http.Client
}

func NewTwilioSMS() *TwilioSMS {
	return &TwilioSMS{
		httpClient: &http.Client{},
	}
}

func (t *TwilioSMS) SendText(ctx context.Context, accessToken, channelExternalID, recipient, body string, attachments []Attachment) (*SendResult, error) {
	// accessToken actually stores "AccountSID:AuthToken" for Twilio
	parts := strings.Split(accessToken, ":")
	if len(parts) != 2 {
		return nil, ErrInvalidCredentials
	}
	accountSID := parts[0]
	authToken := parts[1]

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)

	data := url.Values{}
	data.Set("To", recipient)
	data.Set("From", channelExternalID)
	data.Set("Body", body)

	// If there are attachments, Twilio supports MediaUrl
	for _, att := range attachments {
		if att.URL != "" {
			if strings.HasPrefix(att.URL, "http") {
				data.Add("MediaUrl", att.URL)
			}
			// Note: Local paths won't work unless accessible publicly
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	auth := base64.StdEncoding.EncodeToString([]byte(accountSID + ":" + authToken))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("twilio api error: %d - %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Sid string `json:"sid"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("failed to decode twilio response: %w", err)
	}

	return &SendResult{ExternalID: res.Sid}, nil
}
