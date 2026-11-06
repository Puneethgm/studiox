package identity

import (
	"context"
	"strconv"
	"time"

	"github.com/projectx/api/internal/platform/rediscache"
)

// loginLockoutThreshold/Window: task 4.3's per-account login lockout —
// independent of source IP (keyed by email, not IP, unlike this
// codebase's existing per-IP rate limiter), backed by Redis so it's
// shared across API instances. Same counter shape as
// TwoFALockoutStore, a separate store/key-space because this guards the
// password step, not the 2FA code step.
const loginLockoutThreshold = 5
const loginLockoutWindow = 15 * time.Minute

type LoginLockoutStore struct {
	redis *rediscache.Client
}

func NewLoginLockoutStore(redis *rediscache.Client) *LoginLockoutStore {
	return &LoginLockoutStore{redis: redis}
}

func loginLockoutKey(email string) string { return "login_lockout:" + email }

// RecordFailure increments email's failed-login counter.
func (s *LoginLockoutStore) RecordFailure(ctx context.Context, email string) (justLocked bool, err error) {
	n, err := s.redis.IncrWithTTL(ctx, loginLockoutKey(email), loginLockoutWindow)
	if err != nil {
		return false, err
	}
	return n == loginLockoutThreshold, nil
}

// IsLocked reports whether email's login is currently locked out,
// regardless of which IP the request comes from.
func (s *LoginLockoutStore) IsLocked(ctx context.Context, email string) (bool, error) {
	raw, ok, err := s.redis.Get(ctx, loginLockoutKey(email))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return false, nil
	}
	return n >= loginLockoutThreshold, nil
}

// Reset clears email's failure counter — called on a successful login.
func (s *LoginLockoutStore) Reset(ctx context.Context, email string) error {
	return s.redis.Del(ctx, loginLockoutKey(email))
}
