package identity

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/rediscache"
)

// totpLockoutThreshold/Window: 5 wrong codes locks the code step for
// that account for 15 minutes (platform-security-hardening task 2.4).
const totpLockoutThreshold = 5
const totpLockoutWindow = 15 * time.Minute

// TwoFALockoutStore tracks wrong-code attempts per account (not per IP
// — the design doc's "locks the code step for that account"), backed
// by a single Redis counter whose expiry anchors the 15-minute window
// to the first failure.
type TwoFALockoutStore struct {
	redis *rediscache.Client
}

func NewTwoFALockoutStore(redis *rediscache.Client) *TwoFALockoutStore {
	return &TwoFALockoutStore{redis: redis}
}

func totpLockoutKey(userID uuid.UUID) string { return "totp_lockout:" + userID.String() }

// RecordFailure increments userID's wrong-code counter and reports
// whether this failure is the one that just crossed the lockout
// threshold (so the caller knows to send the one-time lockout
// notification, not one on every subsequent attempt while still locked).
func (s *TwoFALockoutStore) RecordFailure(ctx context.Context, userID uuid.UUID) (justLocked bool, err error) {
	n, err := s.redis.IncrWithTTL(ctx, totpLockoutKey(userID), totpLockoutWindow)
	if err != nil {
		return false, err
	}
	return n == totpLockoutThreshold, nil
}

// IsLocked reports whether userID's code step is currently locked out.
func (s *TwoFALockoutStore) IsLocked(ctx context.Context, userID uuid.UUID) (bool, error) {
	raw, ok, err := s.redis.Get(ctx, totpLockoutKey(userID))
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
	return n >= totpLockoutThreshold, nil
}

// Reset clears userID's failure counter — called on a successful code,
// so a correct code after a few wrong ones doesn't leave a stale
// almost-locked-out counter hanging around for the rest of the window.
func (s *TwoFALockoutStore) Reset(ctx context.Context, userID uuid.UUID) error {
	return s.redis.Del(ctx, totpLockoutKey(userID))
}
