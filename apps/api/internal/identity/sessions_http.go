package identity

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/secaudit"
)

// AdminSessionRoutes mounts the super_admin-only, platform-wide session
// endpoints — mounted under the existing super-admin-only /admin route
// group in cmd/server/main.go, so no additional role check is needed
// here (platform-security-hardening task 1.6).
func (h *Handler) AdminSessionRoutes(r chi.Router) {
	r.Post("/users/{userId}/sessions/end", h.adminForceLogoutUser)
	r.Post("/sessions/end-all", h.adminEndAllSessions)
	r.With(RequireStepUp).Post("/users/{userId}/2fa/reset", h.adminResetUserTOTP)
	r.Get("/2fa-enrollment-stats", h.totpEnrollmentStats)
	r.Get("/security-audit-log", h.listPlatformSecurityAuditLog)
}

// listPlatformSecurityAuditLog godoc
//
//	@Summary		Security audit log, platform-wide (super_admin)
//	@Description	The super_admin read-access path from task 3.6 — unscoped, every studio.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Success		200	{array}	secaudit.LogEntry
//	@Router			/api/v1/admin/security-audit-log [get]
func (h *Handler) listPlatformSecurityAuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := secaudit.ListAll(r.Context(), h.repo.pool, 100)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, entries)
}

// adminForceLogoutUser godoc
//
//	@Summary		Force-log-out any user (super_admin)
//	@Description	Ends every active session for the given user, regardless of which studio they belong to.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Param			userId	path	string	true	"User ID (UUID)"
//	@Success		204		"no content"
//	@Router			/api/v1/admin/users/{userId}/sessions/end [post]
func (h *Handler) adminForceLogoutUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}
	if err := h.RevokeSessionsForUser(r.Context(), userID, "force-logout by super_admin"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.notifySessionsEnded(r.Context(), userID, "an administrator ended your sessions")
	httpx.NoContent(w)
}

// adminEndAllSessions godoc
//
//	@Summary		End every session on the platform (super_admin)
//	@Description	Immediately revokes every active session, platform-wide — the incident-response panic button.
//	@Tags			RBAC
//	@Security		CookieAuth
//	@Success		204	"no content"
//	@Router			/api/v1/admin/sessions/end-all [post]
func (h *Handler) adminEndAllSessions(w http.ResponseWriter, r *http.Request) {
	// Captured BEFORE revoking — RevokeAllSessions only needs session
	// IDs, so this is the one place that knows who's about to be
	// affected, for the notification below (task 1.7).
	affectedUserIDs, err := h.repo.ListUserIDsWithActiveSessions(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.RevokeAllSessions(r.Context(), "platform-wide logout by super_admin"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	for _, userID := range affectedUserIDs {
		h.notifySessionsEnded(r.Context(), userID, "a platform-wide logout was triggered by an administrator")
		// Same clearing as RevokeSessionsForUser (task 2.5) — a
		// platform-wide logout must not leave any remember-browser
		// grant able to skip the code step afterward.
		if err := h.repo.RevokeAllRememberBrowserTokens(r.Context(), userID); err != nil {
			slog.Error("revoke remember-browser tokens failed", "err", err, "user_id", userID)
		}
	}
	// Defense in depth beyond RevokeAllSessions: rotating the signing key
	// and dropping every other one means even a token whose `sessions`
	// row was somehow missed stops verifying at all, not just failing
	// the revocation check (task 1.9). Optional — h.signingKeys is nil
	// unless WithSigningKeyStore was called at startup.
	if h.signingKeys != nil {
		if _, err := h.signingKeys.RotateAndPurge(r.Context()); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpx.NoContent(w)
}

// sessionRes is one session row as shown to the owning user — platform-
// security-hardening task 1.5.
type sessionRes struct {
	ID          uuid.UUID `json:"id"`
	Device      string    `json:"device"`
	Browser     string    `json:"browser"`
	IP          string    `json:"ip"`
	Country     string    `json:"country"`
	TwoFAPassed bool      `json:"twoFaPassed"`
	CreatedAt   string    `json:"createdAt"`
	LastSeenAt  string    `json:"lastSeenAt"`
	IsCurrent   bool      `json:"isCurrent"`
}

func toSessionRes(s *Session, currentID uuid.UUID) sessionRes {
	// s.Device is always empty at write time (CreateSession is called with
	// "" for it — see http.go) and s.Browser holds the full raw
	// User-Agent header, not something fit to show a user. Parse a
	// human-readable (device, browser) pair from it here, at the
	// presentation layer, rather than changing what's stored — the raw
	// value is also what HasPriorSessionFromDevice compares against for
	// new-device detection, which must keep seeing the exact original
	// string.
	device, browser := parseUserAgent(s.Browser)
	return sessionRes{
		ID: s.ID, Device: device, Browser: browser, IP: s.IP, Country: s.Country,
		TwoFAPassed: s.TwoFAPassed,
		CreatedAt:   s.CreatedAt.Format(httpTimeFormat),
		LastSeenAt:  s.LastSeenAt.Format(httpTimeFormat),
		IsCurrent:   s.ID == currentID,
	}
}

const httpTimeFormat = "2006-01-02T15:04:05Z07:00"

// listMySessions godoc
//
//	@Summary		List my active sessions
//	@Description	Every session currently active for the logged-in user, most recently active first, with the session serving this request flagged.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Success		200	{array}	sessionRes
//	@Router			/api/v1/auth/sessions [get]
func (h *Handler) listMySessions(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	sessions, err := h.repo.ListActiveSessions(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	out := make([]sessionRes, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, toSessionRes(s, c.SessionID))
	}
	httpx.JSON(w, http.StatusOK, out)
}

// endMySession godoc
//
//	@Summary		End one of my sessions
//	@Description	Ends the named session immediately — it must belong to the logged-in user. Ending the session serving this very request is allowed (equivalent to logout).
//	@Tags			Auth
//	@Security		CookieAuth
//	@Param			sessionId	path	string	true	"Session ID (UUID)"
//	@Success		204			"no content"
//	@Failure		403			{object}	httpx.ErrorResponse	"session belongs to another user"
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/api/v1/auth/sessions/{sessionId} [delete]
func (h *Handler) endMySession(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid sessionId")
		return
	}
	if !h.endOwnedSession(w, r, sessionID, c.UserID, "ended by user") {
		return
	}
	httpx.NoContent(w)
}

// endOwnedSession looks up sessionID, confirms it belongs to ownerUserID
// (a 403 otherwise — this is the authorization check for every "end a
// specific session" path, self-service or admin), revokes it, and writes
// the appropriate error response itself on failure. Returns false if a
// response was already written (caller should return immediately).
func (h *Handler) endOwnedSession(w http.ResponseWriter, r *http.Request, sessionID, ownerUserID uuid.UUID, reason string) bool {
	s, err := h.repo.GetSession(r.Context(), sessionID)
	if err != nil {
		if err == ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "session not found")
			return false
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return false
	}
	if s.UserID != ownerUserID {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "that session does not belong to this user")
		return false
	}
	if err := h.repo.RevokeSession(r.Context(), sessionID, reason); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return false
	}
	if err := h.sessions.MarkRevoked(r.Context(), sessionID, h.tokens.MaxAge()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return false
	}
	return true
}

// endOtherMySessions godoc
//
//	@Summary		End all my other sessions
//	@Description	Ends every active session for the logged-in user except the one serving this request.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Success		204	"no content"
//	@Router			/api/v1/auth/sessions/end-others [post]
func (h *Handler) endOtherMySessions(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	sessions, err := h.repo.ListActiveSessions(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	for _, s := range sessions {
		if s.ID == c.SessionID {
			continue
		}
		if err := h.repo.RevokeSession(r.Context(), s.ID, "ended by user (end-others)"); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if err := h.sessions.MarkRevoked(r.Context(), s.ID, h.tokens.MaxAge()); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpx.NoContent(w)
}
