package identity

import (
	"net/http"
	"time"

	"github.com/projectx/api/internal/platform/httpx"
)

// totpStaffGracePeriod is how long a studio_staff account may keep
// working without TOTP enrolled before it's blocked — platform-
// security-hardening task 2.9. super_admin and studio_admin get no
// grace at all: enrollment is required at their very next login.
const totpStaffGracePeriod = 14 * 24 * time.Hour

// totpEnforcementAnnouncedAt anchors the grace period for accounts that
// already existed before this feature shipped — without this, every
// pre-existing studio_staff account (created long before today) would
// be retroactively treated as already past a 14-days-from-creation
// deadline and blocked immediately, which is the opposite of a grace
// period. A staff account's actual deadline is
// max(totpEnforcementAnnouncedAt, user.CreatedAt) + totpStaffGracePeriod
// — so a brand-new account still gets the full 14 days from its own
// creation, while an existing one gets 14 days from today.
var totpEnforcementAnnouncedAt = time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

// RequireTOTPEnrollment blocks every other authenticated action until
// the caller has completed TOTP enrollment — immediately for
// super_admin/studio_admin, after the grace period above for
// studio_staff. Mounted the same way as RequirePasswordSet (see
// cmd/server/main.go): login, logout, /auth/me,
// /auth/2fa/setup|confirm|stepup, and the login code-step endpoints are
// all registered before this middleware's router group, so they stay
// reachable even for an unenrolled account — otherwise nobody could
// ever complete enrollment in the first place.
func (h *Handler) RequireTOTPEnrollment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := MustClaims(r.Context())
		enabled, err := h.repo.IsTOTPEnabled(r.Context(), c.UserID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if enabled {
			next.ServeHTTP(w, r)
			return
		}
		if c.Role == RoleStudioStaff {
			u, err := h.repo.FindByID(r.Context(), c.UserID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
			deadline := totpEnforcementAnnouncedAt
			if u.CreatedAt.After(deadline) {
				deadline = u.CreatedAt
			}
			deadline = deadline.Add(totpStaffGracePeriod)
			if time.Now().Before(deadline) {
				next.ServeHTTP(w, r) // still within the grace period
				return
			}
		}
		httpx.WriteError(w, http.StatusForbidden, "totp_enrollment_required", "two-factor authentication enrollment is required")
	})
}
