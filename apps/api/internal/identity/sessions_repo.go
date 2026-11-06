package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Session is one durable login session — see Claims.SessionID's doc
// comment for why this is keyed separately from the JWT's own
// (per-request-rotating) jti. platform-security-hardening task 1.1.
type Session struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Device       string
	Browser      string
	IP           string
	Country      string
	TwoFAPassed  bool
	CreatedAt    time.Time
	LastSeenAt   time.Time
	RevokedAt    *time.Time
	RevokedReason string
}

// IsActive reports whether this session has not been revoked. It does NOT
// check idle/absolute JWT expiry — those live in the token itself
// (Claims.IdleExpiresAt/SessionMaxExpiresAt), not this row.
func (s *Session) IsActive() bool { return s.RevokedAt == nil }

const sessionColumns = `id, user_id, device, browser, ip, country, two_fa_passed, created_at, last_seen_at, revoked_at, revoked_reason`

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	if err := row.Scan(&s.ID, &s.UserID, &s.Device, &s.Browser, &s.IP, &s.Country, &s.TwoFAPassed,
		&s.CreatedAt, &s.LastSeenAt, &s.RevokedAt, &s.RevokedReason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return &s, nil
}

// HasPriorSessionFromDevice reports whether userID has ever had a
// session (active or not) recorded from this exact browser string —
// the new-device-login check behind SendNewDeviceLogin (platform-
// security-hardening task 1.7). Call this BEFORE CreateSession records
// the current login, or every login would trivially "recognize" itself.
func (r *Repo) HasPriorSessionFromDevice(ctx context.Context, userID uuid.UUID, browser string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id = $1 AND browser = $2)`,
		userID, browser,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check prior session from device: %w", err)
	}
	return exists, nil
}

// CreateSession records a new durable login session — called once per
// real re-authentication event (login, password change), not on every
// request (unlike the jti-keyed Redis entry — see SessionStore.Create).
func (r *Repo) CreateSession(ctx context.Context, id, userID uuid.UUID, device, browser, ip, country string, twoFAPassed bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, device, browser, ip, country, two_fa_passed)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, userID, device, browser, ip, country, twoFAPassed)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// TouchSession advances last_seen_at — called on every authenticated
// request (RequireAuth's Refresh path) so the session list shows real
// recent activity.
func (r *Repo) TouchSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// RevokeSession marks one session row revoked. Idempotent — revoking an
// already-revoked session is a no-op, not an error.
func (r *Repo) RevokeSession(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = now(), revoked_reason = $2 WHERE id = $1 AND revoked_at IS NULL
	`, id, reason)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeSessionsForUser marks every one of userID's still-active sessions
// revoked and returns their IDs, so the caller (revokeSessionsForUser in
// sessions.go) can also clear each one's fast Redis revocation check.
func (r *Repo) RevokeSessionsForUser(ctx context.Context, userID uuid.UUID, reason string) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE sessions SET revoked_at = now(), revoked_reason = $2
		WHERE user_id = $1 AND revoked_at IS NULL
		RETURNING id
	`, userID, reason)
	if err != nil {
		return nil, fmt.Errorf("revoke sessions for user: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan revoked session id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListUserIDsWithActiveSessions returns every distinct user who
// currently has at least one active session — used right before
// RevokeAllSessions to know who to notify afterward (platform-security-
// hardening task 1.7), since RevokeAllSessions itself only needs session
// IDs, not who they belong to.
func (r *Repo) ListUserIDsWithActiveSessions(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT user_id FROM sessions WHERE revoked_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list user ids with active sessions: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RevokeAllSessions marks every currently-active session on the platform
// revoked and returns their IDs — the platform-wide "end all sessions"
// panic button (platform-security-hardening task 1.6), for a super_admin
// responding to an incident.
func (r *Repo) RevokeAllSessions(ctx context.Context, reason string) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE sessions SET revoked_at = now(), revoked_reason = $1
		WHERE revoked_at IS NULL
		RETURNING id
	`, reason)
	if err != nil {
		return nil, fmt.Errorf("revoke all sessions: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan revoked session id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListActiveSessions returns userID's currently-active sessions, most
// recently active first — the self-service "my sessions" list.
func (r *Repo) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+sessionColumns+` FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSession fetches one session by ID, active or not — used to check
// ownership (is this session mine / in my studio) before a targeted
// revoke.
func (r *Repo) GetSession(ctx context.Context, id uuid.UUID) (*Session, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id)
	return scanSession(row)
}
