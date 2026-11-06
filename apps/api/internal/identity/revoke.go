package identity

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// notifySessionsEnded emails userID that all of their sessions were just
// ended by someone else (an admin force-logout) or a platform-wide
// logout — platform-security-hardening task 1.7. Best-effort: a failed
// send is logged, never returned as an error that would roll back or
// fail the revocation that already happened.
func (h *Handler) notifySessionsEnded(ctx context.Context, userID uuid.UUID, reason string) {
	if h.mailer == nil || !h.mailer.Enabled() {
		return
	}
	u, err := h.repo.FindByID(ctx, userID)
	if err != nil {
		slog.Error("notifySessionsEnded: lookup user failed", "err", err, "user_id", userID)
		return
	}
	name := ""
	if u.Username != nil {
		name = *u.Username
	}
	if err := h.mailer.SendSessionsEnded(u.Email, name, reason); err != nil {
		slog.Error("send sessions-ended email failed", "err", err, "user_id", userID)
	}
}

// RevokeSessionsForUser ends every active session userID currently has —
// both the durable Postgres record (RevokeSession) and the fast Redis
// check every request goes through (SessionStore.IsRevoked) — so the
// very next request on any of those sessions is rejected, not just their
// next login. platform-security-hardening task 1.3/1.4.
//
// Called from: password change/reset, 2FA reset/disable, role/permission
// change, and user deactivation/removal (task 1.4) — replacing the
// previous "takes effect at next login only" behavior documented at
// roles_http.go's deactivation handler.
func (h *Handler) RevokeSessionsForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	// Clears every "remember this browser" grant too (task 2.5: "cleared
	// on logout-all or password change") — a conservative superset
	// applied to every trigger of this shared helper, not just those
	// two, since a deactivated/role-changed user shouldn't keep a
	// skip-the-code-step grant either.
	if err := h.repo.RevokeAllRememberBrowserTokens(ctx, userID); err != nil {
		return fmt.Errorf("revoke remember-browser tokens: %w", err)
	}
	ids, err := h.repo.RevokeSessionsForUser(ctx, userID, reason)
	if err != nil {
		return fmt.Errorf("revoke sessions in db: %w", err)
	}
	for _, id := range ids {
		if err := h.sessions.MarkRevoked(ctx, id, h.tokens.MaxAge()); err != nil {
			return fmt.Errorf("mark session %s revoked: %w", id, err)
		}
	}
	return nil
}

// RevokeAllSessions ends every active session on the platform — both the
// durable Postgres records and each one's fast Redis check. Platform-
// security-hardening task 1.6, the super_admin "end all sessions" panic
// button.
func (h *Handler) RevokeAllSessions(ctx context.Context, reason string) error {
	ids, err := h.repo.RevokeAllSessions(ctx, reason)
	if err != nil {
		return fmt.Errorf("revoke all sessions in db: %w", err)
	}
	for _, id := range ids {
		if err := h.sessions.MarkRevoked(ctx, id, h.tokens.MaxAge()); err != nil {
			return fmt.Errorf("mark session %s revoked: %w", id, err)
		}
	}
	return nil
}

// revokeSessionsForRole ends every session belonging to every user
// currently assigned roleID — used when a role is deactivated or its
// permissions change (task 1.4), since both are otherwise only reflected
// at each affected user's next login.
func (h *Handler) revokeSessionsForRole(ctx context.Context, roleID uuid.UUID, reason string) error {
	userIDs, err := h.repo.ListUserIDsByRole(ctx, roleID)
	if err != nil {
		return fmt.Errorf("list users for role: %w", err)
	}
	for _, userID := range userIDs {
		if err := h.RevokeSessionsForUser(ctx, userID, reason); err != nil {
			return err
		}
	}
	return nil
}
