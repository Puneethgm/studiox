// Package mistral wraps Mistral's OCR API, used by the Knowledge Base to
// extract text from an uploaded image before it's reviewed and embedded.
// Not part of the chat-reply provider waterfall (see internal/integrations/
// groq, gemini, claude) — Mistral is single-purpose here.
package mistral

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ocrURL    = "https://api.mistral.ai/v1/ocr"
	modelsURL = "https://api.mistral.ai/v1/models"
	ocrModel  = "mistral-ocr-latest"
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 60 * time.Second}}
}

// OCRImage sends raw image bytes to Mistral OCR (base64-encoded inline
// rather than passed as a URL, so this works regardless of whether the
// image is publicly reachable — no dependency on the studio's own upload
// being internet-facing) and returns the extracted text, pages joined with
// blank lines.
func (c *Client) OCRImage(ctx context.Context, apiKey string, imageBytes []byte, contentType string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("mistral api key not configured")
	}
	if contentType == "" {
		contentType = "image/png"
	}

	dataURI := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(imageBytes)
	reqBody, err := json.Marshal(map[string]any{
		"model": ocrModel,
		"document": map[string]any{
			"type":      "image_url",
			"image_url": dataURI,
		},
	})
	if err != nil {
		return "", fmt.Errorf("mistral ocr build request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", fmt.Errorf("mistral ocr new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("mistral ocr request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("mistral ocr read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("mistral ocr status %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Pages []struct {
			Markdown string `json:"markdown"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("mistral ocr parse response: %w", err)
	}

	texts := make([]string, 0, len(out.Pages))
	for _, p := range out.Pages {
		if t := strings.TrimSpace(p.Markdown); t != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("mistral ocr returned no text")
	}
	return strings.Join(texts, "\n\n"), nil
}

// ValidateKey does a cheap auth check (no OCR call) for the Settings page's
// "test key" button — just confirms the key is accepted at all.
func (c *Client) ValidateKey(ctx context.Context, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("mistral api key not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return fmt.Errorf("mistral validate key new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mistral validate key request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mistral validate key status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
