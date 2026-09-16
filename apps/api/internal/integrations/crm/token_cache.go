package crm

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// cachedToken is one connection's currently-live bearer token, obtained via
// a provider's token_exchange login step.
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// tokenCache holds the one live token per crm_connections row, in memory —
// no DB table, since a token is only ever a cheap-to-refetch derivative of a
// connection's real credentials, not data worth persisting or losing sleep
// over on a process restart. Safe for concurrent use across the Executor's
// callers.
type tokenCache struct {
	mu    sync.Mutex
	byKey map[uuid.UUID]cachedToken
}

func newTokenCache() *tokenCache {
	return &tokenCache{byKey: make(map[uuid.UUID]cachedToken)}
}

// get returns the cached token for connectionID if it exists and has not
// expired yet (set already backed off expiresAt by a safety margin, so a
// plain "now after expiresAt" check here is enough — a token never gets
// handed out so close to its real expiry that it could die mid-flight).
func (c *tokenCache) get(connectionID uuid.UUID) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.byKey[connectionID]
	if !ok || time.Now().After(t.expiresAt) {
		return "", false
	}
	return t.token, true
}

// set caches token for the studio's connection, treating it as expiring a
// bit before its real ttl. The margin is capped at a quarter of ttl (rather
// than a fixed few seconds) so a CRM with a short-lived token — a token
// exchange test's few-second stub included — doesn't get treated as
// perpetually expired the instant it's cached.
func (c *tokenCache) set(connectionID uuid.UUID, token string, ttl time.Duration) {
	margin := 5 * time.Second
	if quarter := ttl / 4; quarter < margin {
		margin = quarter
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byKey[connectionID] = cachedToken{token: token, expiresAt: time.Now().Add(ttl - margin)}
}
