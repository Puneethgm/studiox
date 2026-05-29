package studios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/secrets"
)

type SocialWorker struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
	log    *slog.Logger
}

func NewSocialWorker(pool *pgxpool.Pool, encryptionKey string, log *slog.Logger) *SocialWorker {
	var cipher *secrets.Cipher
	if encryptionKey != "" {
		var err error
		cipher, err = secrets.New(encryptionKey)
		if err != nil {
			log.Error("failed to initialize secrets cipher for social worker", "err", err)
		}
	}

	return &SocialWorker{
		pool:   pool,
		cipher: cipher,
		log:    log,
	}
}

func (w *SocialWorker) Run(ctx context.Context) {
	w.log.Info("social publishing worker started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("social publishing worker stopping")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *SocialWorker) tick(ctx context.Context) {
	// Find scheduled posts that are due
	now := time.Now().UTC()
	rows, err := w.pool.Query(ctx, `
		SELECT id, studio_id, campaign, platform, copy, media_url, status, scheduled_at
		FROM social_posts
		WHERE status = 'scheduled' AND scheduled_at <= $1
	`, now)
	if err != nil {
		w.log.Error("query due social posts", "err", err)
		return
	}
	defer rows.Close()

	var posts []SocialPost
	for rows.Next() {
		var p SocialPost
		err := rows.Scan(&p.ID, &p.StudioID, &p.Campaign, &p.Platform, &p.Copy, &p.MediaURL, &p.Status, &p.ScheduledAt)
		if err != nil {
			w.log.Error("scan social post", "err", err)
			continue
		}
		posts = append(posts, p)
	}
	rows.Close()

	for _, post := range posts {
		w.publishPost(ctx, post)
	}
}

func (w *SocialWorker) publishPost(ctx context.Context, post SocialPost) {
	w.log.Info("publishing social post", "id", post.ID, "platform", post.Platform, "campaign", post.Campaign)

	if post.Platform == "Google Ads" {
		w.publishGoogleAds(ctx, post)
		return
	}
	if post.Platform == "X (Twitter)" {
		w.publishX(ctx, post)
		return
	}

	// Fetch connected Meta Channel Account for Page ID & Access Token
	var pageID string
	var accessTokenEnc string
	err := w.pool.QueryRow(ctx, `
		SELECT external_id, access_token_enc
		FROM channel_accounts
		WHERE studio_id = $1 AND kind = 'messenger_meta' AND status = 'active'
		LIMIT 1
	`, post.StudioID).Scan(&pageID, &accessTokenEnc)

	isLocalDev := os.Getenv("API_ENV") == "local"

	if err != nil {
		if isLocalDev {
			// In local dev, allow publishing using fallback credentials/mocking
			w.log.Info("[MOCK PUBLISH] No active Meta integration found. Simulating publish in local dev.", "post_id", post.ID)
			w.markStatus(ctx, post.ID, "published")
			return
		}

		w.log.Error("no active Facebook page connected for studio", "studio_id", post.StudioID, "post_id", post.ID)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// Decrypt Access Token
	var accessToken string
	if w.cipher != nil && accessTokenEnc != "" {
		dec, err := w.cipher.Decrypt(accessTokenEnc)
		if err != nil {
			if isLocalDev {
				w.log.Warn("failed to decrypt meta token in local dev; falling back to mock mode", "err", err)
				accessToken = "test"
			} else {
				w.log.Error("decrypt meta token", "err", err, "post_id", post.ID)
				w.markStatus(ctx, post.ID, "failed")
				return
			}
		} else {
			accessToken = dec
		}
	}

	// If in local dev with test/empty token, mock the API call
	if isLocalDev && (accessToken == "" || accessToken == "test") {
		w.log.Info("[MOCK PUBLISH] Successfully published scheduled post to Facebook Page feed",
			"page_id", pageID,
			"copy", post.Copy,
			"media", post.MediaURL,
		)
		w.markStatus(ctx, post.ID, "published")
		return
	}

	// Make Meta Graph API Request
	err = w.sendToFacebook(ctx, pageID, accessToken, post.Copy, post.MediaURL)
	if err != nil {
		w.log.Error("failed to publish to facebook", "err", err, "post_id", post.ID)
		if isLocalDev {
			w.log.Warn("[LOCAL DEV FALLBACK] Meta API call failed (likely due to missing App permissions like pages_manage_posts). Marking post as published for local UI testing.", "post_id", post.ID)
			w.markStatus(ctx, post.ID, "published")
			return
		}
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	w.log.Info("successfully published to facebook page", "post_id", post.ID)
	w.markStatus(ctx, post.ID, "published")
}

func (w *SocialWorker) markStatus(ctx context.Context, postID uuid.UUID, status string) {
	_, err := w.pool.Exec(ctx, `
		UPDATE social_posts
		SET status = $1, updated_at = now()
		WHERE id = $2
	`, status, postID)
	if err != nil {
		w.log.Error("update social post status", "err", err, "post_id", postID, "status", status)
	}
}

func (w *SocialWorker) publishX(ctx context.Context, post SocialPost) {
	var accessTokenEnc string
	err := w.pool.QueryRow(ctx, `
		SELECT access_token_enc
		FROM channel_accounts
		WHERE studio_id = $1 AND kind = 'x_dm' AND status = 'active'
		LIMIT 1
	`, post.StudioID).Scan(&accessTokenEnc)

	isLocalDev := os.Getenv("API_ENV") == "local"

	if err != nil {
		if isLocalDev {
			w.log.Info("[MOCK PUBLISH] No active X integration found. Simulating publish in local dev.", "post_id", post.ID)
			w.markStatus(ctx, post.ID, "published")
			return
		}
		w.log.Error("no active X account connected for studio", "studio_id", post.StudioID, "post_id", post.ID)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	var accessTokenStr string
	if w.cipher != nil && accessTokenEnc != "" {
		dec, err := w.cipher.Decrypt(accessTokenEnc)
		if err != nil {
			if isLocalDev {
				accessTokenStr = "test"
			} else {
				w.log.Error("decrypt x token", "err", err, "post_id", post.ID)
				w.markStatus(ctx, post.ID, "failed")
				return
			}
		} else {
			accessTokenStr = dec
		}
	}

	if isLocalDev && (accessTokenStr == "" || accessTokenStr == "test") {
		w.log.Info("[MOCK PUBLISH] Successfully published scheduled post to X", "copy", post.Copy)
		w.markStatus(ctx, post.ID, "published")
		return
	}

	var keys struct {
		ConsumerKey       string `json:"consumer_key"`
		ConsumerSecret    string `json:"consumer_secret"`
		AccessToken       string `json:"access_token"`
		AccessTokenSecret string `json:"access_token_secret"`
	}
	if err := json.Unmarshal([]byte(accessTokenStr), &keys); err != nil {
		w.log.Error("invalid x channel credentials", "err", err, "post_id", post.ID)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	config := oauth1.NewConfig(keys.ConsumerKey, keys.ConsumerSecret)
	token := oauth1.NewToken(keys.AccessToken, keys.AccessTokenSecret)
	httpClient := config.Client(ctx, token)

	// Twitter API v2 Tweet Endpoint
	endpoint := "https://api.twitter.com/2/tweets"
	payload := map[string]interface{}{
		"text": post.Copy,
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	var errBody []byte
	if resp != nil && resp.Body != nil {
		errBody, _ = io.ReadAll(resp.Body)
	}

	if err != nil || resp.StatusCode >= 300 {
		w.log.Error("failed to publish to x", "err", err, "status", resp.StatusCode, "body", string(errBody))
		w.markStatus(ctx, post.ID, "failed")
		return
	}
	defer resp.Body.Close()

	w.log.Info("successfully published to x", "post_id", post.ID, "status", resp.StatusCode, "body", string(errBody))
	w.markStatus(ctx, post.ID, "published")
}

func (w *SocialWorker) sendToFacebook(ctx context.Context, pageID, accessToken, message, mediaURL string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	var apiURL string
	formValues := url.Values{}
	formValues.Set("access_token", accessToken)

	hasImage := false
	if mediaURL != "" {
		ext := strings.ToLower(filepath.Ext(mediaURL))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif" {
			hasImage = true
		}
	}

	// If there's an image attachment, upload it to page photos
	if hasImage {
		apiURL = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/photos", pageID)
		// Meta Graph API requires public accessibility of URL, or we can fallback to feed link
		if strings.HasPrefix(mediaURL, "http") {
			formValues.Set("url", mediaURL)
		} else {
			// For local uploads paths (/uploads/...), we construct base URL if configured
			baseURL := os.Getenv("API_BASE_URL")
			if baseURL == "" {
				baseURL = "http://localhost:8080"
			}
			formValues.Set("url", baseURL+mediaURL)
		}
		formValues.Set("caption", message)
	} else {
		// Text/link post to Page feed
		apiURL = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/feed", pageID)
		formValues.Set("message", message)
		if mediaURL != "" {
			if strings.HasPrefix(mediaURL, "http") {
				formValues.Set("link", mediaURL)
			} else {
				baseURL := os.Getenv("API_BASE_URL")
				if baseURL == "" {
					baseURL = "http://localhost:8080"
				}
				formValues.Set("link", baseURL+mediaURL)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(formValues.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
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
		return fmt.Errorf("facebook api response: HTTP %d, error: %s (code %d)", resp.StatusCode, errEnv.Error.Message, errEnv.Error.Code)
	}

	return nil
}

func (w *SocialWorker) publishGoogleAds(ctx context.Context, post SocialPost) {
	w.log.Info("publishing social post to Google Ads", "id", post.ID, "campaign", post.Campaign)

	isLocalDev := os.Getenv("API_ENV") == "local"

	// Fetch connected Google Ads Channel Account for Customer ID & Refresh Token
	var customerID string
	var refreshTokenEnc string
	err := w.pool.QueryRow(ctx, `
		SELECT external_id, access_token_enc
		FROM channel_accounts
		WHERE studio_id = $1 AND kind = 'google_ads' AND status = 'active'
		LIMIT 1
	`, post.StudioID).Scan(&customerID, &refreshTokenEnc)

	if err != nil {
		if isLocalDev {
			w.log.Info("[MOCK GOOGLE ADS PUBLISH] No active Google Ads integration found. Simulating campaign publishing in local dev.", "post_id", post.ID)
			w.markStatus(ctx, post.ID, "published")
			return
		}
		w.log.Error("no active Google Ads account connected for studio", "studio_id", post.StudioID, "post_id", post.ID)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// Fetch Google Ads API credentials from the studio table
	var clientID, clientSecret, devToken string
	err = w.pool.QueryRow(ctx, `
		SELECT google_client_id, google_client_secret, google_developer_token
		FROM studios
		WHERE id = $1
	`, post.StudioID).Scan(&clientID, &clientSecret, &devToken)

	if err != nil || clientID == "" || clientSecret == "" || devToken == "" {
		if isLocalDev {
			w.log.Info("[MOCK GOOGLE ADS PUBLISH] Incomplete studio credentials. Simulating campaign publishing in local dev.", "post_id", post.ID)
			w.markStatus(ctx, post.ID, "published")
			return
		}
		w.log.Error("incomplete Google Ads API credentials in studio settings", "studio_id", post.StudioID, "post_id", post.ID, "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// Decrypt Refresh Token
	var refreshToken string
	if w.cipher != nil && refreshTokenEnc != "" {
		dec, err := w.cipher.Decrypt(refreshTokenEnc)
		if err != nil {
			if isLocalDev {
				w.log.Warn("failed to decrypt google ads refresh token in local dev; falling back to mock mode", "err", err)
				refreshToken = "test"
			} else {
				w.log.Error("decrypt google ads refresh token", "err", err, "post_id", post.ID)
				w.markStatus(ctx, post.ID, "failed")
				return
			}
		} else {
			refreshToken = dec
		}
	}

	if isLocalDev && refreshToken == "test" {
		w.log.Info("[MOCK GOOGLE ADS PUBLISH] Successfully published Google Ads campaign to Manager Account",
			"customer_id", customerID,
			"campaign_name", post.Campaign,
			"copy", post.Copy,
		)
		w.markStatus(ctx, post.ID, "published")
		return
	}

	// 1. Refresh Access Token
	tokenURL := "https://oauth2.googleapis.com/token"
	formValues := url.Values{}
	formValues.Set("client_id", clientID)
	formValues.Set("client_secret", clientSecret)
	formValues.Set("refresh_token", refreshToken)
	formValues.Set("grant_type", "refresh_token")

	resp, err := http.PostForm(tokenURL, formValues)
	if err != nil {
		w.log.Error("failed to refresh google ads oauth token", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		w.log.Error("google ads oauth refresh error", "status", resp.StatusCode, "body", string(respBody))
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		w.log.Error("failed to decode google ads oauth token", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	accessToken := tokenRes.AccessToken

	// Clean customer ID format (remove hyphens)
	cleanCustomerID := strings.ReplaceAll(customerID, "-", "")

	// 2. Create Campaign Budget
	budgetResourceName, err := w.createGoogleAdsBudget(ctx, cleanCustomerID, accessToken, devToken, post.Campaign)
	if err != nil {
		w.log.Error("failed to create google ads budget", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// 3. Create Campaign
	campaignResourceName, err := w.createGoogleAdsCampaign(ctx, cleanCustomerID, accessToken, devToken, post.Campaign, budgetResourceName)
	if err != nil {
		w.log.Error("failed to create google ads campaign", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// 4. Create Ad Group
	adGroupResourceName, err := w.createGoogleAdsAdGroup(ctx, cleanCustomerID, accessToken, devToken, post.Campaign, campaignResourceName)
	if err != nil {
		w.log.Error("failed to create google ads ad group", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	// 5. Create Responsive Search Ad (Ad Group Ad)
	err = w.createGoogleAdsResponsiveSearchAd(ctx, cleanCustomerID, accessToken, devToken, post.Copy, adGroupResourceName)
	if err != nil {
		w.log.Error("failed to create google ads responsive search ad", "err", err)
		w.markStatus(ctx, post.ID, "failed")
		return
	}

	w.log.Info("successfully published Google Ads campaign hierarchy", "post_id", post.ID, "campaign", post.Campaign)
	w.markStatus(ctx, post.ID, "published")
}

func (w *SocialWorker) createGoogleAdsBudget(ctx context.Context, customerID, accessToken, devToken, campaignName string) (string, error) {
	apiURL := fmt.Sprintf("https://googleads.googleapis.com/v17/customers/%s/campaignBudgets:mutate", customerID)
	payload := map[string]any{
		"operations": []map[string]any{
			{
				"create": map[string]any{
					"name":                  fmt.Sprintf("%s Budget %d", campaignName, time.Now().Unix()),
					"amountMicros":          10000000, // $10.00 USD daily budget
					"deliveryMethod":        "STANDARD",
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("developer-token", devToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("mutate budget API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var mutateRes struct {
		Results []struct {
			ResourceName string `json:"resourceName"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &mutateRes); err != nil {
		return "", err
	}
	if len(mutateRes.Results) == 0 {
		return "", fmt.Errorf("no budget resource name returned")
	}
	return mutateRes.Results[0].ResourceName, nil
}

func (w *SocialWorker) createGoogleAdsCampaign(ctx context.Context, customerID, accessToken, devToken, campaignName, budgetResourceName string) (string, error) {
	apiURL := fmt.Sprintf("https://googleads.googleapis.com/v17/customers/%s/campaigns:mutate", customerID)
	payload := map[string]any{
		"operations": []map[string]any{
			{
				"create": map[string]any{
					"name":               fmt.Sprintf("%s %d", campaignName, time.Now().Unix()),
					"advertisingChannelType": "SEARCH",
					"status":             "PAUSED",
					"campaignBudget":     budgetResourceName,
					"manualCpc":          map[string]any{},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("developer-token", devToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("mutate campaign API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var mutateRes struct {
		Results []struct {
			ResourceName string `json:"resourceName"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &mutateRes); err != nil {
		return "", err
	}
	if len(mutateRes.Results) == 0 {
		return "", fmt.Errorf("no campaign resource name returned")
	}
	return mutateRes.Results[0].ResourceName, nil
}

func (w *SocialWorker) createGoogleAdsAdGroup(ctx context.Context, customerID, accessToken, devToken, campaignName, campaignResourceName string) (string, error) {
	apiURL := fmt.Sprintf("https://googleads.googleapis.com/v17/customers/%s/adGroups:mutate", customerID)
	payload := map[string]any{
		"operations": []map[string]any{
			{
				"create": map[string]any{
					"name":         fmt.Sprintf("%s Ad Group", campaignName),
					"status":       "PAUSED",
					"campaign":     campaignResourceName,
					"type":         "SEARCH_STANDARD",
					"cpcBidMicros": 1000000, // $1.00 Max CPC Bid
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("developer-token", devToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("mutate ad group API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var mutateRes struct {
		Results []struct {
			ResourceName string `json:"resourceName"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &mutateRes); err != nil {
		return "", err
	}
	if len(mutateRes.Results) == 0 {
		return "", fmt.Errorf("no ad group resource name returned")
	}
	return mutateRes.Results[0].ResourceName, nil
}

func (w *SocialWorker) createGoogleAdsResponsiveSearchAd(ctx context.Context, customerID, accessToken, devToken, copyText, adGroupResourceName string) error {
	apiURL := fmt.Sprintf("https://googleads.googleapis.com/v17/customers/%s/adGroupAds:mutate", customerID)

	// Build headlines & descriptions
	headline1 := "Join Our Premium Studio"
	headline2 := "Special Fitness Offer"
	headline3 := "Book A Slot Today"

	desc1 := copyText
	if len(desc1) > 90 {
		desc1 = desc1[:87] + "..."
	}
	desc2 := "Experience the best athletic community and custom training schedules built for you."

	payload := map[string]any{
		"operations": []map[string]any{
			{
				"create": map[string]any{
					"adGroup": adGroupResourceName,
					"status":  "PAUSED",
					"ad": map[string]any{
						"finalUrls": []string{"https://studiox.fit"},
						"responsiveSearchAd": map[string]any{
							"headlines": []map[string]any{
								{"text": headline1},
								{"text": headline2},
								{"text": headline3},
							},
							"descriptions": []map[string]any{
								{"text": desc1},
								{"text": desc2},
							},
						},
					},
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("developer-token", devToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("mutate ad group ad API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}
