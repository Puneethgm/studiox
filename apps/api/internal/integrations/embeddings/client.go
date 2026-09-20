// Package embeddings talks to the local Python embedding service
// (apps/embeddings) that replaced Gemini's embedContent API for the RAG/
// knowledge-base and semantic-search pipelines. Unlike the Gemini/Groq
// clients, there's no API key — the service is a local sidecar reachable
// only from this process, identified purely by base URL.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	url  string
	http *http.Client
}

// New returns nil if baseURL is empty, mirroring groq.New/claude.New's
// "unconfigured means nil" convention — callers guard with `c == nil`.
func New(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{
		url: baseURL,
		// Longer than Gemini's 15s: the first request after the Python
		// process starts pays a one-time model-load cost (can take from a
		// few seconds up to over a minute on a cold cache). Steady-state
		// single-text CPU inference is well under a second.
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

type embedRequest struct {
	Text    string `json:"text"`
	IsQuery bool   `json:"is_query"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed calls the local embedding service. isQuery selects the e5
// "query: "/"passage: " prefix server-side — callers never build the
// prefixed string themselves. Prefer EmbedQuery/EmbedPassage below so each
// call site states its intent explicitly.
func (c *Client) Embed(ctx context.Context, text string, isQuery bool) ([]float32, error) {
	if c == nil {
		return nil, fmt.Errorf("embeddings client not configured")
	}

	reqBody, err := json.Marshal(embedRequest{Text: text, IsQuery: isQuery})
	if err != nil {
		return nil, fmt.Errorf("embeddings marshal request: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/embed", bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("embeddings new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("embeddings request: %w", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("embeddings status %d: %s", resp.StatusCode, string(body))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("embeddings status %d: %s", resp.StatusCode, string(body))
		}

		var out embedResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, fmt.Errorf("embeddings parse response: %w", err)
		}
		if len(out.Embedding) == 0 {
			return nil, fmt.Errorf("embeddings: empty embedding response")
		}
		return out.Embedding, nil
	}
	return nil, lastErr
}

// EmbedQuery embeds text that will be used to search — e.g. an expanded
// customer query, right before a vector similarity search.
func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return c.Embed(ctx, text, true)
}

// EmbedPassage embeds text that is being stored as a future search target —
// e.g. a knowledge-base chunk, a message body, or a staff reply.
func (c *Client) EmbedPassage(ctx context.Context, text string) ([]float32, error) {
	return c.Embed(ctx, text, false)
}
