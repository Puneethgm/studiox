package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

func newTOTPHTTPTestHandler(t *testing.T) (*Handler, uuid.UUID) {
	t.Helper()
	repo, _, _ := testRepo(t)
	h := &Handler{repo: repo, cipher: testCipher(t)}
	userID := testUserID(t, repo)
	return h, userID
}

func TestSetupThenConfirmTOTP_EnrollsAndReturnsRecoveryCodesOnce(t *testing.T) {
	h, userID := newTOTPHTTPTestHandler(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID)
	})

	setupReq := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/setup", nil), &Claims{UserID: userID})
	setupRec := httptest.NewRecorder()
	h.setupTOTP(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200, body=%s", setupRec.Code, setupRec.Body.String())
	}
	var setupRes setupTOTPRes
	if err := json.Unmarshal(setupRec.Body.Bytes(), &setupRes); err != nil {
		t.Fatalf("unmarshal setup response: %v", err)
	}
	if setupRes.Secret == "" || !strings.HasPrefix(setupRes.URI, "otpauth://") {
		t.Fatalf("unexpected setup response: %+v", setupRes)
	}

	code, err := totp.GenerateCode(setupRes.Secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	confirmBody, _ := json.Marshal(confirmTOTPReq{Code: code})
	confirmReq := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", strings.NewReader(string(confirmBody))), &Claims{UserID: userID})
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	h.confirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200, body=%s", confirmRec.Code, confirmRec.Body.String())
	}
	var codesRes recoveryCodesRes
	if err := json.Unmarshal(confirmRec.Body.Bytes(), &codesRes); err != nil {
		t.Fatalf("unmarshal confirm response: %v", err)
	}
	if len(codesRes.RecoveryCodes) != recoveryCodeCount {
		t.Fatalf("got %d recovery codes, want %d", len(codesRes.RecoveryCodes), recoveryCodeCount)
	}

	enabled, err := h.repo.IsTOTPEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("IsTOTPEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected TOTP to be enabled after a successful confirm")
	}
	count, err := h.repo.CountUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes: %v", err)
	}
	if count != recoveryCodeCount {
		t.Errorf("CountUnusedRecoveryCodes = %d, want %d", count, recoveryCodeCount)
	}
}

func TestConfirmTOTP_WrongCodeRejectedAndDoesNotEnable(t *testing.T) {
	h, userID := newTOTPHTTPTestHandler(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID) })

	setupReq := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/setup", nil), &Claims{UserID: userID})
	setupRec := httptest.NewRecorder()
	h.setupTOTP(setupRec, setupReq)
	if setupRec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, want 200", setupRec.Code)
	}

	confirmBody, _ := json.Marshal(confirmTOTPReq{Code: "000000"})
	confirmReq := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", strings.NewReader(string(confirmBody))), &Claims{UserID: userID})
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	h.confirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("confirm (wrong code) status = %d, want 422, body=%s", confirmRec.Code, confirmRec.Body.String())
	}

	enabled, err := h.repo.IsTOTPEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("IsTOTPEnabled: %v", err)
	}
	if enabled {
		t.Error("expected TOTP to remain disabled after a failed confirm")
	}
}

func TestConfirmTOTP_WithoutSetupReturnsBadRequest(t *testing.T) {
	h, userID := newTOTPHTTPTestHandler(t)

	confirmBody, _ := json.Marshal(confirmTOTPReq{Code: "123456"})
	confirmReq := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/confirm", strings.NewReader(string(confirmBody))), &Claims{UserID: userID})
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	h.confirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", confirmRec.Code, confirmRec.Body.String())
	}
}
