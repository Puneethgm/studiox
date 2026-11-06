package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/rediscache"
)

// ErrSessionNotFound means jti has no active session in Redis — either it
// was never created, it was revoked (logout, password change), or its idle
// window expired.
var ErrSessionNotFound = errors.New("session not found or expired")

// SessionStore tracks active sessions by jti in Redis, purely for
// revocation — logout and password-change delete a jti's key immediately,
// so that specific token stops working even though its own expiry hasn't
// passed yet. (Idle timeout is enforced by the JWT itself now — see
// Claims.IdleExpiresAt and TokenIssuer.Refresh — not by this store's TTL;
// the TTL here just self-cleans an abandoned session's Redis entry rather
// than letting it linger forever.)
//
// Because RequireAuth rotates the token (and therefore the jti) on every
// authenticated request, this store sees a new Create on every request,
// not just at login — that's the cost of baking the idle timeout into the
// token itself rather than relying solely on Redis's TTL. Deliberately NOT
// paired with a Revoke of the old jti: a single page load routinely fires
// more than one authenticated request concurrently (parallel data
// fetching, prefetch, retries), all still carrying whatever cookie the
// browser had before any of them got a response — revoking the old jti
// the instant the first one rotates would 401 the others. The old jti is
// simply left to expire on its own TTL.
//
// Unlike a perf cache, this is an access-control mechanism: a Redis
// outage here fails CLOSED — every call returns an error, and RequireAuth rejects the
// request — rather than silently letting a revoked session keep working
// for the rest of its JWT lifetime.
type SessionStore struct {
	redis *rediscache.Client
	idle  time.Duration
}

// NewSessionStore builds a SessionStore whose Redis entries live for
// idleTimeout — long enough to outlast the same duration's idle window
// baked into each issued token, so a session's Redis record and its
// token-level idle deadline expire at roughly the same time.
func NewSessionStore(redis *rediscache.Client, idleTimeout time.Duration) *SessionStore {
	return &SessionStore{redis: redis, idle: idleTimeout}
}

func sessionKey(jti string) string { return "session:" + jti }

// Create registers jti as a newly issued, active session for userID,
// starting with a full idle-timeout window.
func (s *SessionStore) Create(ctx context.Context, jti string, userID uuid.UUID) error {
	return s.redis.Set(ctx, sessionKey(jti), []byte(userID.String()), s.idle)
}

// Exists reports whether jti still has an active (non-revoked, non-expired)
// session. Called on every authenticated request before RequireAuth
// rotates the token — see Handler.RequireAuth. Returns ErrSessionNotFound
// if jti has no active session.
func (s *SessionStore) Exists(ctx context.Context, jti string) error {
	_, ok, err := s.redis.Get(ctx, sessionKey(jti))
	if err != nil {
		return err
	}
	if !ok {
		return ErrSessionNotFound
	}
	return nil
}

// Revoke ends jti's session immediately — logout, password change, or any
// other forced re-issue — regardless of its remaining idle window or JWT
// expiry. Revoking an already-absent jti is a no-op, not an error.
func (s *SessionStore) Revoke(ctx context.Context, jti string) error {
	return s.redis.Del(ctx, sessionKey(jti))
}

// revokedSessionKey is deliberately a different prefix from sessionKey —
// the two serve different lookups (jti liveness vs. durable-session
// revocation, see Claims.SessionID's doc comment) and must never collide.
func revokedSessionKey(sessionID uuid.UUID) string { return "revoked_session:" + sessionID.String() }

// MarkRevoked records sessionID as revoked for ttl — the fast,
// every-request check RequireAuth makes via IsRevoked, backed by the
// `sessions` table (identity.Repo) as the durable source of truth. ttl
// should be at least the longest possible remaining token lifetime
// (SessionMaxExpiresAt - now) so a revoked session can't become
// "un-revoked" by the Redis key simply expiring while the token itself
// would still otherwise verify.
func (s *SessionStore) MarkRevoked(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error {
	return s.redis.Set(ctx, revokedSessionKey(sessionID), []byte("1"), ttl)
}

// IsRevoked reports whether sessionID has been marked revoked. Like
// Exists, this fails CLOSED: a Redis error is returned to the caller
// (which must treat it as "reject the request"), never silently treated
// as "not revoked".
func (s *SessionStore) IsRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	_, ok, err := s.redis.Get(ctx, revokedSessionKey(sessionID))
	if err != nil {
		return false, err
	}
	return ok, nil
}
