package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// rememberBrowserTTL is the "remember this browser" grant's lifetime —
// studio_admin/studio_staff only, never super_admin (platform-security-
// hardening task 2.5).
const rememberBrowserTTL = 14 * 24 * time.Hour

// rememberBrowserCookieName is deliberately distinct from both session
// cookies (px_session, px_csrf) — this one long-outlives any single
// session and is read only at the password step of login, before any
// session exists yet.
const rememberBrowserCookieName = "px_remember"

// CreateRememberBrowserToken mints and stores a new remember-browser
// grant for userID, returning the raw token to set as a cookie value —
// same hash-only-at-rest pattern as CreatePasswordResetToken.
func (r *Repo) CreateRememberBrowserToken(ctx context.Context, userID uuid.UUID) (rawToken string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate remember-browser token: %w", err)
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	_, err = r.pool.Exec(ctx, `
		INSERT INTO remember_browser_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + $3)
	`, userID, hash, rememberBrowserTTL)
	if err != nil {
		return "", fmt.Errorf("insert remember-browser token: %w", err)
	}
	return rawToken, nil
}

// IsRememberBrowserTokenValid reports whether rawToken is a current,
// unexpired, unrevoked remember-browser grant for userID — the check
// login's password step makes to decide whether the code step can be
// skipped.
func (r *Repo) IsRememberBrowserTokenValid(ctx context.Context, userID uuid.UUID, rawToken string) (bool, error) {
	if rawToken == "" {
		return false, nil
	}
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM remember_browser_tokens
			WHERE user_id = $1 AND token_hash = $2 AND revoked_at IS NULL AND expires_at > now()
		)
	`, userID, hash).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check remember-browser token: %w", err)
	}
	return exists, nil
}

// RevokeAllRememberBrowserTokens invalidates every remember-browser
// grant for userID — called on logout-all and on password change
// (task 2.5: "cleared on logout-all or password change").
func (r *Repo) RevokeAllRememberBrowserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE remember_browser_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("revoke remember-browser tokens: %w", err)
	}
	return nil
}
