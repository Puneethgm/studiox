package groq

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://api.groq.com/openai/v1/chat/completions"

const (
	Model8B  = "llama-3.1-8b-instant"
	Model70B = "llama-3.3-70b-versatile"
)

type Client struct {
	key  string
	http *http.Client
}

func New(key string) *Client {
	if key == "" {
		return nil
	}
	return &Client{key: key, http: &http.Client{Timeout: 30 * time.Second}}
}

// GenerateReply calls the Groq API with the given model and returns the text response.
func (c *Client) GenerateReply(ctx context.Context, prompt, model string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("groq client not configured")
	}
	if model == "" {
		model = Model8B
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  512,
		"temperature": 0.7,
	})

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL, strings.NewReader(string(reqBody)))
		if err != nil {
			return "", fmt.Errorf("groq new request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("groq request: %w", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("groq status %d: %s", resp.StatusCode, string(body))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("groq status %d: %s", resp.StatusCode, string(body))
		}

		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return "", fmt.Errorf("groq parse response: %w", err)
		}
		if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
			return "", fmt.Errorf("groq empty response")
		}
		return strings.TrimSpace(out.Choices[0].Message.Content), nil
	}
	return "", lastErr
}
