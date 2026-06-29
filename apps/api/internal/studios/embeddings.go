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
	"unicode"
)

// ChunkTextSemantic splits text on semantic boundaries (paragraphs → sentences)
// instead of raw character counts. This preserves meaning at chunk edges.
// maxChars is a soft limit — a sentence that exceeds it is kept whole.
// overlap is the number of characters carried from the previous chunk.
func ChunkTextSemantic(text string, maxChars int, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= maxChars {
		return []string{text}
	}

	// Split into paragraphs first, then sentences within each paragraph.
	paragraphs := strings.Split(text, "\n\n")
	var sentences []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sentences = append(sentences, splitSentences(p)...)
	}

	var chunks []string
	var current strings.Builder
	var tail string // overlap tail from previous chunk

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
			// Carry the last `overlap` chars into the next chunk
			runes := []rune(s)
			if len(runes) > overlap {
				tail = string(runes[len(runes)-overlap:])
			} else {
				tail = s
			}
		}
		current.Reset()
		if tail != "" {
			current.WriteString(tail)
			current.WriteString(" ")
		}
	}

	for _, sent := range sentences {
		tentative := current.String() + sent
		if len([]rune(tentative)) > maxChars && current.Len() > 0 {
			flush()
		}
		current.WriteString(sent)
		current.WriteString(" ")
	}
	flush()

	return chunks
}

// splitSentences splits a paragraph into individual sentences using punctuation boundaries.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Peek ahead: if next char is space or end, it's a sentence boundary.
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// ChunkText is the legacy fixed-size chunker kept for backward compatibility.
func ChunkText(text string, maxChars int, overlap int) []string {
	return ChunkTextSemantic(text, maxChars, overlap)
}

// ClassifyIntent calls Gemini to classify the intent and sentiment of a message.
// Returns intent label, sentiment (-1/0/1), confidence (0.0–1.0).
// Falls back to neutral/unknown on error so the pipeline never blocks.
func ClassifyIntent(ctx context.Context, apiKey, message string) (intent string, sentiment int, confidence float64) {
	intent = "unknown"
	sentiment = 0
	confidence = 0.5

	prompt := fmt.Sprintf(`You are a fitness studio CRM classifier. Classify this customer message.

Message: "%s"

Reply with ONLY valid JSON — no markdown, no explanation:
{
  "intent": "<one of: pricing_question | booking_inquiry | objection | ready_to_buy | general_question | off_topic>",
  "sentiment": <-1 for negative, 0 for neutral, 1 for positive>,
  "confidence": <float 0.0 to 1.0>
}`, message)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     0.0,
			"maxOutputTokens": 80,
		},
	})
	if err != nil {
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode >= 400 {
		return
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		return
	}

	raw := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	// Strip markdown code fences if present
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result struct {
		Intent     string  `json:"intent"`
		Sentiment  int     `json:"sentiment"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return
	}

	return result.Intent, result.Sentiment, result.Confidence
}

// RerankChunks uses Gemini to rerank retrieved chunks by actual relevance
// to the query. Returns chunks sorted best-first (up to topK).
func RerankChunks(ctx context.Context, apiKey, query string, chunks []string, topK int) []string {
	if len(chunks) == 0 {
		return chunks
	}
	if len(chunks) <= 1 {
		return chunks
	}

	var chunkList strings.Builder
	for i, c := range chunks {
		chunkList.WriteString(fmt.Sprintf("[%d] %s\n\n", i, c))
	}

	prompt := fmt.Sprintf(`You are a relevance ranker for a fitness studio FAQ system.

Customer query: "%s"

Candidate passages:
%s

Return ONLY a JSON array of passage indices sorted by relevance (most relevant first), e.g. [2,0,1].
Include only the top %d most relevant. No explanation.`, query, chunkList.String(), topK)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	reqBody, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     0.0,
			"maxOutputTokens": 50,
		},
	})

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return chunks[:min(topK, len(chunks))]
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return chunks[:min(topK, len(chunks))]
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode >= 400 {
		return chunks[:min(topK, len(chunks))]
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		return chunks[:min(topK, len(chunks))]
	}
	if len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return chunks[:min(topK, len(chunks))]
	}

	raw := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var indices []int
	if err := json.Unmarshal([]byte(raw), &indices); err != nil {
		return chunks[:min(topK, len(chunks))]
	}

	var reranked []string
	seen := make(map[int]bool)
	for _, idx := range indices {
		if idx >= 0 && idx < len(chunks) && !seen[idx] {
			reranked = append(reranked, chunks[idx])
			seen[idx] = true
		}
	}
	// Fill remaining slots if reranker returned fewer than topK
	for i, c := range chunks {
		if len(reranked) >= topK {
			break
		}
		if !seen[i] {
			reranked = append(reranked, c)
		}
	}
	return reranked
}

// GetGeminiEmbedding calls Gemini gemini-embedding-2 and returns a 768-dim vector.
func GetGeminiEmbedding(ctx context.Context, apiKey string, text string) ([]float32, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:embedContent?key=%s",
		apiKey,
	)

	reqBody, err := json.Marshal(map[string]any{
		"model": "models/gemini-embedding-2",
		"content": map[string]any{
			"parts": []map[string]any{{"text": text}},
		},
		"outputDimensionality": 768,
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

// ExpandQuery calls Gemini 2.5 Flash to generate 3 alternative phrasings of the
// query, then prepends the original query and returns all 4 joined by a space.
// On any error the original query is returned unchanged.
func ExpandQuery(ctx context.Context, apiKey, query string) string {
	prompt := fmt.Sprintf(`You are a search query optimizer for a fitness studio. Generate 3 alternative phrasings of this customer query that capture the same intent but use different vocabulary. Return ONLY a JSON array of strings, no markdown. Query: "%s"`, query)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)

	reqBody, err := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]any{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     0.0,
			"maxOutputTokens": 120,
		},
	})
	if err != nil {
		return query
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return query
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return query
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode >= 400 {
		return query
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		return query
	}

	raw := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var alternatives []string
	if err := json.Unmarshal([]byte(raw), &alternatives); err != nil {
		return query
	}

	all := append([]string{query}, alternatives...)
	return strings.Join(all, " ")
}

// MaxMarginalRelevance selects topK candidates using Maximal Marginal Relevance.
// It balances relevance to the query against diversity among selected candidates.
// lambdaMmr controls the trade-off: 1.0 = pure relevance, 0.0 = pure diversity.
// Similarity is computed as the dot product (Gemini embeddings are unit-norm).
// If len(candidates) <= topK the full slice is returned as-is.
func MaxMarginalRelevance(query []float32, candidates []string, embeddings [][]float32, topK int, lambdaMmr float32) []string {
	n := len(candidates)
	if n <= topK {
		return candidates
	}

	dotProduct := func(a, b []float32) float32 {
		var sum float32
		l := len(a)
		if len(b) < l {
			l = len(b)
		}
		for i := 0; i < l; i++ {
			sum += a[i] * b[i]
		}
		return sum
	}

	// Pre-compute similarity of every candidate to the query.
	querySims := make([]float32, n)
	for i := range candidates {
		querySims[i] = dotProduct(query, embeddings[i])
	}

	selected := make([]int, 0, topK)
	remaining := make([]int, n)
	for i := range remaining {
		remaining[i] = i
	}

	for len(selected) < topK && len(remaining) > 0 {
		bestScore := float32(-1e9)
		bestPos := 0

		for pos, idx := range remaining {
			var score float32
			if len(selected) == 0 {
				// First pick: pure relevance.
				score = querySims[idx]
			} else {
				// MMR score: lambda * sim(query, c) - (1-lambda) * max(sim(c, s))
				maxSimToSelected := float32(-1e9)
				for _, selIdx := range selected {
					s := dotProduct(embeddings[idx], embeddings[selIdx])
					if s > maxSimToSelected {
						maxSimToSelected = s
					}
				}
				score = lambdaMmr*querySims[idx] - (1-lambdaMmr)*maxSimToSelected
			}
			if score > bestScore {
				bestScore = score
				bestPos = pos
			}
		}

		chosen := remaining[bestPos]
		selected = append(selected, chosen)
		// Remove chosen from remaining by swapping with last element.
		remaining[bestPos] = remaining[len(remaining)-1]
		remaining = remaining[:len(remaining)-1]
	}

	result := make([]string, len(selected))
	for i, idx := range selected {
		result[i] = candidates[idx]
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
