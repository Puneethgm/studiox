package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// CaptchaVerifier is task 8.3's bot-protection check for the login
// endpoint, beyond the existing per-IP rate limiter
// (httpx.AuthRateLimiter). Behind an interface so tests use a fake
// rather than calling a real third-party CAPTCHA provider.
type CaptchaVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

// TurnstileVerifier calls Cloudflare Turnstile's siteverify API — a
// real implementation, not a stub, but NOT wired up live in this
// sandbox: there is no real Turnstile site key/secret available here
// (it requires registering a real site with Cloudflare), so
// h.captcha is left nil in cmd/server/main.go. When configured, a
// missing/invalid token on the login request fails verification and
// the caller should reject the login attempt before touching the DB.
type TurnstileVerifier struct {
	secret string
	client *http.Client
}

func NewTurnstileVerifier(secret string) *TurnstileVerifier {
	return &TurnstileVerifier{secret: secret, client: &http.Client{Timeout: 3 * time.Second}}
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, nil
	}
	form := url.Values{"secret": {v.secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		nil)
	if err != nil {
		return false, fmt.Errorf("build turnstile request: %w", err)
	}
	req.URL.RawQuery = form.Encode()

	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("call turnstile: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode turnstile response: %w", err)
	}
	return result.Success, nil
}
