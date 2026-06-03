package studios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChunkText splits text into chunks of ~maxChars with overlap characters of context
// carried over to the next chunk, preventing context loss at boundaries.
func ChunkText(text string, maxChars int, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	n := len(runes)

	for i := 0; i < n; {
		end := i + maxChars
		if end > n {
			end = n
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == n {
			break
		}
		i = end - overlap
		if i < 0 {
			i = 0
		}
		if i >= end {
			i = end // guard against infinite loop
		}
	}
	return chunks
}

// GetGeminiEmbedding calls Gemini text-embedding-004 and returns a 768-dim vector.
func GetGeminiEmbedding(ctx context.Context, apiKey string, text string) ([]float32, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=%s",
		apiKey,
	)

	reqBody, err := json.Marshal(map[string]any{
		"model": "models/text-embedding-004",
		"content": map[string]any{
			"parts": []map[string]any{{"text": text}},
		},
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gemini embedding API error (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, err
	}
	if len(res.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding response from Gemini API")
	}
	return res.Embedding.Values, nil
}

// FormatVectorAsString converts []float32 → "[v1,v2,...]" for pgvector text input.
func FormatVectorAsString(vec []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", v)
	}
	sb.WriteByte(']')
	return sb.String()
}
