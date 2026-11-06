package identity

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/secaudit"
)

// verifyEnabledTOTPCode validates code against userID's already-enabled
// TOTP secret, rejecting a replay via the stored last-used step
// (task 2.3's "a 30-second-old code already used once is rejected on
// reuse"), and persists the new step on success.
func (h *Handler) verifyEnabledTOTPCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	rec, err := h.repo.GetTOTP(ctx, userID)
	if err != nil {
		return false, err
	}
	if !rec.Enabled {
		return false, ErrTOTPNotEnrolled
	}
	secret, err := h.cipher.Decrypt(rec.SecretCiphertext)
	if err != nil {
		return false, err
	}
	ok, step, err := validateTOTPCode(code, secret, time.Now())
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if rec.LastUsedStep != nil && step <= *rec.LastUsedStep {
		return false, nil // replay of an already-accepted step
	}
	if err := h.repo.MarkTOTPStepUsed(ctx, userID, step); err != nil {
		return false, err
	}
	return true, nil
}

// setupTOTPRes is what the enrollment-start step returns — enough for a
// frontend to render a QR code (from uri) or offer manual entry (secret).
type setupTOTPRes struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// setupTOTP godoc
//
//	@Summary		Start TOTP enrollment
//	@Description	Generates a fresh TOTP secret and stores it pending (not yet active) — call confirmTOTP with a code from it to finish enrolling. Calling this again before confirming replaces the pending secret.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Success		200	{object}	setupTOTPRes
//	@Router			/api/v1/auth/2fa/setup [post]
func (h *Handler) setupTOTP(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	u, err := h.repo.FindByID(r.Context(), c.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	secret, uri, err := generateTOTPSecret(u.Email)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	ciphertext, err := h.cipher.Encrypt(secret)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.repo.SaveTOTPSecretPending(r.Context(), c.UserID, ciphertext); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, setupTOTPRes{Secret: secret, URI: uri})
}

type confirmTOTPReq struct {
	Code string `json:"code"`
}

// recoveryCodesRes is deliberately its own response shape (not reused
// anywhere else) so it's obvious at a glance which endpoints ever return
// plaintext recovery codes — exactly these two, exactly once each
// (task 2.2's "shown once").
type recoveryCodesRes struct {
	RecoveryCodes []string `json:"recoveryCodes"`
}

// confirmTOTP godoc
//
//	@Summary		Confirm TOTP enrollment
//	@Description	Validates code against the pending secret from setupTOTP; on success, enables TOTP and returns 10 recovery codes — shown exactly this once, never retrievable again.
//	@Tags			Auth
//	@Security		CookieAuth
//	@Param			body	body	confirmTOTPReq	true	"6-digit code from the authenticator app"
//	@Success		200		{object}	recoveryCodesRes
//	@Failure		400		{object}	httpx.ErrorResponse	"wrong code, or no pending enrollment"
//	@Router			/api/v1/auth/2fa/confirm [post]
func (h *Handler) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	c := MustClaims(r.Context())
	var req confirmTOTPReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	rec, err := h.repo.GetTOTP(r.Context(), c.UserID)
	if err != nil {
		if err == ErrTOTPNotEnrolled {
			httpx.WriteError(w, http.StatusBadRequest, "not_pending", "call setup first")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	secret, err := h.cipher.Decrypt(rec.SecretCiphertext)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	ok, step, err := validateTOTPCode(req.Code, secret, time.Now())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if !ok {
		httpx.WriteValidationError(w, map[string]string{"code": "invalid code"})
		return
	}

	if err := h.repo.EnableTOTP(r.Context(), c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := h.repo.MarkTOTPStepUsed(r.Context(), c.UserID, step); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	codes, err := generateRecoveryCodes()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	hashes := make([]string, len(codes))
	for i, code := range codes {
		hash, hashErr := HashPassword(normalizeRecoveryCode(code))
		if hashErr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		hashes[i] = hash
	}
	if err := h.repo.CreateRecoveryCodes(r.Context(), c.UserID, hashes); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), nil, "2fa.enabled", secaudit.OutcomeSuccess, nil)

	httpx.JSON(w, http.StatusOK, recoveryCodesRes{RecoveryCodes: codes})
}
