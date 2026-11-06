package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
)

func TestRequireStepUp_RejectsWithNoVerificationEver(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := RequireStepUp(next)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/x", nil), &Claims{TwoFAVerifiedAt: nil})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireStepUp_AcceptsWithinFreshnessWindowRejectsOutside(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := RequireStepUp(next)

	recent := jwtNumericDateNow(t, -2*time.Minute)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/x", nil), &Claims{TwoFAVerifiedAt: recent})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status (2 min ago) = %d, want 200", rec.Code)
	}

	stale := jwtNumericDateNow(t, -11*time.Minute)
	req2 := withClaims(httptest.NewRequest(http.MethodPost, "/x", nil), &Claims{TwoFAVerifiedAt: stale})
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("status (11 min ago) = %d, want 403", rec2.Code)
	}
}

func TestStepUp_CorrectCodeRefreshesSessionAndPassesRequireStepUp(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)
	secret := enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID) })

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	body, _ := json.Marshal(stepUpReq{Code: code})
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/stepup", strings.NewReader(string(body))), &Claims{UserID: userID})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.stepUp(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	var newCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "px_session" {
			newCookie = c
		}
	}
	if newCookie == nil {
		t.Fatal("expected a refreshed session cookie")
	}
	claims, err := h.tokens.Parse(ctx, newCookie.Value)
	if err != nil {
		t.Fatalf("parse refreshed token: %v", err)
	}
	if claims.TwoFAVerifiedAt == nil || time.Since(claims.TwoFAVerifiedAt.Time) > time.Minute {
		t.Error("expected the refreshed token's TwoFAVerifiedAt to be very recent")
	}
}

func TestStepUp_WrongCodeDoesNotRefreshSession(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)
	enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_ = h.totpLockout.Reset(ctx, userID)
	})

	body, _ := json.Marshal(stepUpReq{Code: "000000"})
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/2fa/stepup", strings.NewReader(string(body))), &Claims{UserID: userID})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.stepUp(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("expected no session cookie to be set on a wrong step-up code")
	}
}

// jwtNumericDateNow returns a *jwt.NumericDate offset from now by d, as
// Claims.TwoFAVerifiedAt expects.
func jwtNumericDateNow(t *testing.T, d time.Duration) *jwt.NumericDate {
	t.Helper()
	return jwt.NewNumericDate(time.Now().Add(d))
}
