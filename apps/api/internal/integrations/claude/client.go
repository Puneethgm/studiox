package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultAPIURL = "https://api.anthropic.com/v1/messages"
const defaultModel = "claude-3-5-sonnet-latest"

type Client struct {
	url  string
	key  string
	http *http.Client
}

func New(url, key string) (*Client, error) {
	if url == "" || key == "" {
		if key == "" {
			return nil, nil
		}
		url = defaultAPIURL
	}
	return &Client{url: url, key: key, http: &http.Client{Timeout: 20 * time.Second}}, nil
}

// GenerateReply sends a prompt to the Claude endpoint and returns the text reply.
func (c *Client) GenerateReply(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", errors.New("claude client not configured")
	}
	reqBody := map[string]any{
		"model":      defaultModel,
		"max_tokens": 512,
		"messages": []map[string]any{{
			"role":    "user",
			"content": prompt,
		}},
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, strings.NewReader(string(b)))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("claude status %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse a few common shapes. First, { "completion": "..." }
	var out map[string]any
	if err := json.Unmarshal(body, &out); err == nil {
		if arr, ok := out["content"].([]any); ok {
			parts := make([]string, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if s, ok := m["text"].(string); ok && s != "" {
						parts = append(parts, s)
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, ""), nil
			}
		}
		if v, ok := out["completion"].(string); ok && v != "" {
			return v, nil
		}
		if v, ok := out["text"].(string); ok && v != "" {
			return v, nil
		}
		if v, ok := out["output"]; ok {
			if s, ok := v.(string); ok {
				return s, nil
			}
			// sometimes output is array
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					return s, nil
				}
			}
		}
	}

	// fallback: return raw body as string
	return string(body), nil
}
