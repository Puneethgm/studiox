package identity

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/riskalert"
	"github.com/projectx/api/internal/platform/secaudit"
)

// resetUserTOTP wipes userID's TOTP enrollment and recovery codes,
// ends all of their sessions, and notifies them — the common tail of
// both reset-chain tiers (task 2.8).
func (h *Handler) resetUserTOTP(w http.ResponseWriter, r *http.Request, userID uuid.UUID) bool {
	if err := h.repo.DisableTOTP(r.Context(), userID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return false
	}
	if err := h.RevokeSessionsForUser(r.Context(), userID, "2FA reset by admin"); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return false
	}
	// Fills task 2.8's deferred TODO now that group 3's secaudit.Logger
	// exists. actor is the admin resetting it (from claims), not userID
	// (the target) — see the two callers below for who that is at each
	// reset-chain tier.
	if c, ok := ClaimsFrom(r.Context()); ok {
		h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), nil, "2fa.reset_by_admin", secaudit.OutcomeSuccess, map[string]any{"targetUserId": userID})
	}
	if riskalert.TwoFADisabled() {
		h.sendRiskAlert("2FA disabled", fmt.Sprintf("Two-factor authentication was reset/disabled for user %s.", userID))
	}
	h.notifySessionsEnded(r.Context(), userID, "your two-factor authentication was reset by an administrator — you'll need to enroll again at your next login")
	return true
}

// resetStudioUserTOTP godoc
//
//	@Summary		Reset a teammate's 2FA (studio_admin)
//	@Description	Lost-phone admin path, tier 1: studio_admin resets a studio_staff member's 2FA. Requires a fresh step-up code from the caller. Ends all of the target's sessions and emails them.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Param			userId		path	string	true	"User ID (UUID)"
//	@Success		204			"no content"
//	@Failure		403			{object}	httpx.ErrorResponse	"target is not a studio_staff member of this studio, or is the caller themselves"
//	@Router			/api/v1/studios/{studioId}/users/{userId}/2fa/reset [post]
func (h *Handler) resetStudioUserTOTP(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}
	if userID == c.UserID {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "cannot reset your own 2FA this way")
		return
	}
	target, err := h.repo.FindByID(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	// This tier is studio_admin -> studio_staff only (design.md's reset
	// chain) — a studio_admin or super_admin account, even if it
	// happens to carry this studioId, must go through the super_admin
	// tier (adminResetUserTOTP) instead.
	if target.StudioID == nil || *target.StudioID != studioID || target.Role != RoleStudioStaff {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "target is not a studio_staff member of this studio")
		return
	}
	if h.resetUserTOTP(w, r, userID) {
		httpx.NoContent(w)
	}
}

// adminResetUserTOTP godoc
//
//	@Summary		Reset any user's 2FA (super_admin)
//	@Description	Lost-phone admin path, tiers 2+3: super_admin resets a studio_admin's or another super_admin's 2FA. A super_admin can never reset their own. Requires a fresh step-up code from the caller.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Param			userId	path	string	true	"User ID (UUID)"
//	@Success		204		"no content"
//	@Failure		403		{object}	httpx.ErrorResponse	"cannot reset your own 2FA"
//	@Router			/api/v1/admin/users/{userId}/2fa/reset [post]
func (h *Handler) adminResetUserTOTP(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid userId")
		return
	}
	if userID == c.UserID {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "a super_admin cannot reset their own 2FA — ask a second super_admin")
		return
	}
	if h.resetUserTOTP(w, r, userID) {
		httpx.NoContent(w)
	}
}
