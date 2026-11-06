package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/rediscache"
)

// ErrPendingAuthNotFound means the pending token is unknown, already
// consumed, or its 5-minute window has elapsed.
var ErrPendingAuthNotFound = errors.New("pending auth token not found or expired")

// pendingAuthTTL is the window between the password step and the code
// step (task 2.3) — a user who doesn't enter a code within this long
// must restart from the password step.
const pendingAuthTTL = 5 * time.Minute

// PendingAuthStore holds the brief, password-verified-but-not-yet-fully-
// authenticated state between login's two steps. Deliberately NOT a
// session — it carries no claims, no permissions, nothing RequireAuth
// would accept; it only proves "this request already supplied the
// correct password for this user, within the last 5 minutes."
type PendingAuthStore struct {
	redis *rediscache.Client
}

func NewPendingAuthStore(redis *rediscache.Client) *PendingAuthStore {
	return &PendingAuthStore{redis: redis}
}

func pendingAuthKey(token string) string { return "pending_2fa:" + token }

// Create mints a fresh pending token for userID, valid for
// pendingAuthTTL.
func (s *PendingAuthStore) Create(ctx context.Context, userID uuid.UUID) (token string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	if err := s.redis.Set(ctx, pendingAuthKey(token), []byte(userID.String()), pendingAuthTTL); err != nil {
		return "", err
	}
	return token, nil
}

// Resolve returns the userID a still-valid pending token was issued
// for. Does NOT consume the token — callers that are about to complete
// the flow (a correct code) should call Consume instead; a wrong-code
// attempt should leave the token alive so the user can retry within the
// same 5-minute window.
func (s *PendingAuthStore) Resolve(ctx context.Context, token string) (uuid.UUID, error) {
	raw, ok, err := s.redis.Get(ctx, pendingAuthKey(token))
	if err != nil {
		return uuid.Nil, err
	}
	if !ok {
		return uuid.Nil, ErrPendingAuthNotFound
	}
	userID, err := uuid.Parse(string(raw))
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// Consume resolves token (same as Resolve) and deletes it — single use,
// called once a code step actually succeeds.
func (s *PendingAuthStore) Consume(ctx context.Context, token string) (uuid.UUID, error) {
	userID, err := s.Resolve(ctx, token)
	if err != nil {
		return uuid.Nil, err
	}
	_ = s.redis.Del(ctx, pendingAuthKey(token))
	return userID, nil
}
