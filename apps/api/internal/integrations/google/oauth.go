package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/messaging"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/studios"
)

type OAuthHandler struct {
	studiosSvc *studios.Service
	msgRepo    *messaging.Repo
	baseURL    string // e.g. "https://1herosocial.ai" or ngrok url
}

func NewOAuthHandler(studiosSvc *studios.Service, msgRepo *messaging.Repo, baseURL string) *OAuthHandler {
	return &OAuthHandler{
		studiosSvc: studiosSvc,
		msgRepo:    msgRepo,
		baseURL:    baseURL,
	}
}

// Redirect URL will be <baseURL>/api/v1/auth/google/callback
func (h *OAuthHandler) getRedirectURI() string {
	return strings.TrimSuffix(h.baseURL, "/") + "/api/v1/auth/google/callback"
}

// LoginHandler initiates the Google OAuth 2.0 flow
func (h *OAuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	studioIDStr := chi.URLParam(r, "studioId")
	if studioIDStr == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_studio", "studioId is required in URL")
		return
	}
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_studio", "invalid studioId format")
		return
	}

	// Verify user has access to this studio
	claims := identity.MustClaims(r.Context())
	if !claims.IsSuper() && (claims.StudioID == nil || *claims.StudioID != studioID) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "access denied to studio")
		return
	}

	// Fetch studio to get Google Client ID
	studio, err := h.studiosSvc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to fetch studio")
		return
	}

	if studio.GoogleClientID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_credentials", "Google Client ID not configured for this studio")
		return
	}

	// Generate random state
	b := make([]byte, 16)
	rand.Read(b)
	stateToken := base64.URLEncoding.EncodeToString(b)
	
	// In a real app, we would save stateToken -> studioID in redis or db with a TTL.
	// For simplicity, we can pass the studioID inside the state as well: stateToken|studioID
	state := fmt.Sprintf("%s|%s", stateToken, studioID.String())

	// Build OAuth URL
	u, _ := url.Parse("https://accounts.google.com/o/oauth2/v2/auth")
	q := u.Query()
	q.Set("client_id", studio.GoogleClientID)
	q.Set("redirect_uri", h.getRedirectURI())
	q.Set("response_type", "code")
	q.Set("scope", "https://www.googleapis.com/auth/adwords")
	q.Set("access_type", "offline")
	q.Set("prompt", "consent") // Force consent to get refresh token
	q.Set("state", state)
	u.RawQuery = q.Encode()

	httpx.JSON(w, http.StatusOK, map[string]string{"url": u.String()})
}

// CallbackHandler handles the redirect from Google
func (h *OAuthHandler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errMsg := r.URL.Query().Get("error")

	if errMsg != "" {
		httpx.WriteError(w, http.StatusBadRequest, "oauth_error", "Google OAuth returned an error: "+errMsg)
		return
	}

	if code == "" || state == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Missing code or state")
		return
	}

	// Extract studio ID from state
	parts := strings.Split(state, "|")
	if len(parts) != 2 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "Malformed state parameter")
		return
	}
	studioID, err := uuid.Parse(parts[1])
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "Invalid studio ID in state")
		return
	}

	// Fetch studio for client secret
	studio, err := h.studiosSvc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Failed to fetch studio")
		return
	}

	// Exchange code for token
	tokenRes, err := exchangeCodeForToken(r.Context(), studio.GoogleClientID, studio.GoogleClientSecret, code, h.getRedirectURI())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Failed to exchange code for token: "+err.Error())
		return
	}

	// Save refresh token to channel_accounts
	_, err = h.msgRepo.CreateChannel(r.Context(), messaging.CreateChannelInput{
		StudioID:      studioID,
		Kind:          "google_ads",
		BSP:           "google",
		ExternalID:    studio.GoogleClientID,
		DisplayHandle: "Google Ads",
		AccessToken:   tokenRes.RefreshToken, // We save the refresh token since it's long-lived
	})
	if err != nil {
		// If it's already connected, we could update it. For now just handle the error.
		if strings.Contains(err.Error(), "already connected") {
			// Ignore or update
		} else {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Failed to save Google Ads channel: "+err.Error())
			return
		}
	}

	// Success response. The popup can be closed via a simple HTML script.
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<html>
		<body>
			<script>
				window.opener && window.opener.location.reload();
				window.close();
			</script>
			<h2>OAuth Successful!</h2>
			<p>You can close this window.</p>
		</body>
		</html>
	`))
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func exchangeCodeForToken(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google token endpoint returned status: %d", resp.StatusCode)
	}

	var res TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token returned (ensure prompt=consent)")
	}

	return &res, nil
}
