// Package gemini calls Google's Gemini API. Extracted from
// internal/messaging/ai_worker.go's former generateGeminiReply/tryGeminiModel
// methods so it can be reused outside the messaging chat-reply pipeline
// (e.g. internal/integrations/llm's provider abstraction) without
// duplicating the HTTP/retry logic. Unlike claude.Client and groq.Client,
// there's no fixed API key at construction time — Gemini keys are
// studio-configurable (BYO key), so the key is a per-call parameter, same
// as the original ai_worker.go methods.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Models is the fallback order tried by GenerateReply — a newer model can
// be rate-limited or briefly unavailable, so the next one in line is tried
// automatically rather than failing the whole request.
var Models = []string{"gemini-2.5-flash", "gemini-2.0-flash", "gemini-2.0-flash-lite"}

// Reply bundles the text response with token usage from the Gemini API.
type Reply struct {
	Text      string
	TokensIn  int
	TokensOut int
}

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{}}
}

// GenerateReply tries each model in Models in order, returning the first
// successful reply.
func (c *Client) GenerateReply(ctx context.Context, apiKey, prompt string) (Reply, error) {
	var lastErr error
	for _, model := range Models {
		r, err := c.tryModel(ctx, apiKey, model, prompt)
		if err == nil {
			return r, nil
		}
		lastErr = err
	}
	return Reply{}, fmt.Errorf("all Gemini models failed: %w", lastErr)
}

func (c *Client) tryModel(ctx context.Context, apiKey, model, prompt string) (Reply, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": prompt}}},
		},
	})
	if err != nil {
		return Reply{}, err
	}

	var lastErr error
	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= 3; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewBuffer(reqBody))
		if err != nil {
			cancel()
			return Reply{}, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = err
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return Reply{}, lastErr
		}

		var res struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(respBytes, &res); err != nil {
			return Reply{}, err
		}
		if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
			return Reply{}, fmt.Errorf("empty response from Gemini API")
		}
		return Reply{
			Text:      res.Candidates[0].Content.Parts[0].Text,
			TokensIn:  res.UsageMetadata.PromptTokenCount,
			TokensOut: res.UsageMetadata.CandidatesTokenCount,
		}, nil
	}

	return Reply{}, fmt.Errorf("gemini API call failed after 3 attempts: %w", lastErr)
}
