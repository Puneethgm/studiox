package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/platform/rediscache"
)

// AnswerCache caches the AI waterfall's answer (see llmWaterfall and the
// inline waterfall in handleMessage) per studio, so an identical prompt for
// the same studio is served from Redis instead of paying for and waiting on
// another round through Groq/Gemini/Claude. kind namespaces the different
// callers (test chat, style-profile rebuild, conversation summary, template
// generation, live incoming-message replies) so they can never collide even
// if two of them happened to build byte-identical prompts.
//
// A nil *AnswerCache (e.g. in tests that don't wire Redis) is always a miss
// on Get and a no-op on Set — callers never need to nil-check first.
type AnswerCache struct {
	client *rediscache.Client
	ttl    time.Duration
}

// NewAnswerCache builds a cache that stores entries for ttl. client may be
// nil (caching is then disabled everywhere it's used).
func NewAnswerCache(client *rediscache.Client, ttl time.Duration) *AnswerCache {
	return &AnswerCache{client: client, ttl: ttl}
}

type cachedAnswer struct {
	Text      string `json:"text"`
	SourceRef string `json:"source_ref"`
}

// answerCacheKey scopes the key by studio first, so two studios can never
// read each other's cached answer even if the rest of the key is identical.
func answerCacheKey(studioID uuid.UUID, kind, prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("studio:%s:ai:%s:%x", studioID, kind, sum)
}

// Get returns the cached (text, sourceRef) for this studio+kind+prompt, or
// ok=false on a miss — including when the cache is disabled (nil) or the
// backend errors, so callers can always fall through to calling the model.
func (c *AnswerCache) Get(ctx context.Context, studioID uuid.UUID, kind, prompt string) (text, sourceRef string, ok bool) {
	if c == nil || c.client == nil {
		return "", "", false
	}
	raw, hit, err := c.client.Get(ctx, answerCacheKey(studioID, kind, prompt))
	if err != nil || !hit {
		return "", "", false
	}
	var v cachedAnswer
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", "", false
	}
	return v.Text, v.SourceRef, true
}

// Set stores a successful answer. It is a best-effort write: a disabled
// cache, an empty text (nothing to cache), or a backend error are all
// silently ignored — a cache write must never fail the caller's request.
func (c *AnswerCache) Set(ctx context.Context, studioID uuid.UUID, kind, prompt, text, sourceRef string) {
	if c == nil || c.client == nil || text == "" {
		return
	}
	raw, err := json.Marshal(cachedAnswer{Text: text, SourceRef: sourceRef})
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, answerCacheKey(studioID, kind, prompt), raw, c.ttl)
}
