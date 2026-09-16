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
const defaultModel = "claude-haiku-4-5-20251001"

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

// Reply bundles the text response with token usage from the Claude API.
type Reply struct {
	Text      string
	TokensIn  int
	TokensOut int
}

// GenerateReply sends a prompt to the Claude endpoint and returns text + token counts.
// A thin wrapper over do() with the original small-chat-reply defaults —
// unchanged so every existing call site (internal/messaging/ai_worker.go)
// keeps behaving exactly as before.
func (c *Client) GenerateReply(ctx context.Context, prompt string) (Reply, error) {
	return c.do(ctx, "", prompt, 512, defaultModel)
}

// Analyze sends a system prompt + document to Claude with a token budget
// sized for structured extraction over a full document (e.g. an uploaded
// OpenAPI spec), not a short chat reply. model is the caller's choice
// (typically resolved per-task via internal/integrations/llm) — empty falls
// back to defaultModel.
func (c *Client) Analyze(ctx context.Context, systemPrompt, document, model string) (Reply, error) {
	if model == "" {
		model = defaultModel
	}
	return c.do(ctx, systemPrompt, document, 4096, model)
}

func (c *Client) do(ctx context.Context, systemPrompt, userPrompt string, maxTokens int, model string) (Reply, error) {
	if c == nil {
		return Reply{}, errors.New("claude client not configured")
	}
	reqBody := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages": []map[string]any{{
			"role":    "user",
			"content": userPrompt,
		}},
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, strings.NewReader(string(b)))
	if err != nil {
		return Reply{}, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("x-api-key", c.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Reply{}, fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Reply{}, fmt.Errorf("claude status %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Completion string `json:"completion"`
		Text       string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Reply{Text: string(body)}, nil
	}

	r := Reply{TokensIn: out.Usage.InputTokens, TokensOut: out.Usage.OutputTokens}

	if len(out.Content) > 0 {
		parts := make([]string, 0, len(out.Content))
		for _, c := range out.Content {
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		r.Text = strings.Join(parts, "")
	} else if out.Completion != "" {
		r.Text = out.Completion
	} else if out.Text != "" {
		r.Text = out.Text
	} else {
		r.Text = string(body)
	}
	return r, nil
}
