package studios

import (
	"fmt"
	"strconv"
	"strings"
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

// ParseVectorString reverses FormatVectorAsString — reads a pgvector
// column's text form ("[v1,v2,...]", via an `embedding::text` select) back
// into []float32. Used to reuse an existing chunk's embedding when its
// content hasn't changed since the last knowledge-base sync, instead of
// re-calling the embeddings API for content that's identical to what's
// already stored (see asyncSyncKnowledgeChunks).
func ParseVectorString(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	vec := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parse vector component %d: %w", i, err)
		}
		vec[i] = float32(f)
	}
	return vec, nil
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
