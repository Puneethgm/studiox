package identity

import (
	"net/http"
	"time"

	"github.com/projectx/api/internal/platform/httpx"
)

// stepUpFreshness is how recently a code must have been verified for
// RequireStepUp to pass — platform-security-hardening task 2.6.
const stepUpFreshness = 10 * time.Minute

// RequireStepUp is middleware for sensitive actions (secrets/
// integrations, exports, admin add/remove, role changes, payment-link
// changes, 2FA toggle, studio deletion — task 2.6's list). It requires
// claims.TwoFAVerifiedAt to be within the last 10 minutes; a user with
// no 2FA enrolled at all (TwoFAVerifiedAt always nil) is always denied
// here — sensitive actions require 2FA to exist, not just a password.
func RequireStepUp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := MustClaims(r.Context())
		if c.TwoFAVerifiedAt == nil || time.Since(c.TwoFAVerifiedAt.Time) > stepUpFreshness {
			httpx.WriteError(w, http.StatusForbidden, "step_up_required", "enter a fresh authenticator code to continue")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type stepUpReq struct {
	Code string `json:"code"`
}

// stepUp godoc
//
//	@Summary		Step-up reauthentication
//	@Description	Checks a fresh TOTP code for the already-authenticated caller and, on success, refreshes the session so RequireStepUp-gated actions work for the next 10 minutes. Does not accept a recovery code — see task 2.6.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Accept			json
//	@Param			body	body	stepUpReq	true	"6-digit code"
//	@Success		204		"no content"
//	@Failure		422		{object}	httpx.ErrorResponse	"wrong code"
//	@Router			/api/v1/auth/2fa/stepup [post]
func (h *Handler) stepUp(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	var req stepUpReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	if h.totpLockout != nil {
		locked, err := h.totpLockout.IsLocked(r.Context(), c.UserID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		if locked {
			httpx.WriteError(w, http.StatusLocked, "locked", "too many wrong codes — try again in 15 minutes")
			return
		}
	}

	ok, err := h.verifyEnabledTOTPCode(r.Context(), c.UserID, req.Code)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !ok {
		if h.totpLockout != nil {
			if _, lockErr := h.totpLockout.RecordFailure(r.Context(), c.UserID); lockErr != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}
		}
		httpx.WriteValidationError(w, map[string]string{"code": "invalid code"})
		return
	}
	if h.totpLockout != nil {
		_ = h.totpLockout.Reset(r.Context(), c.UserID)
	}

	u, err := h.repo.FindByID(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	now := time.Now()
	if err := h.issueSessionCookie(w, r, u, true, &now); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.NoContent(w)
}
