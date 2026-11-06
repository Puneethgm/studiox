package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin_CaptchaRejectsInvalidToken(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	h.captcha = &fakeCaptchaVerifier{validTokens: map[string]bool{"good-token": true}}
	email, password, _ := createTestLoginUser(t, h)

	body, _ := json.Marshal(loginReq{Email: email, Password: password, CaptchaToken: "wrong-token"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.login(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (bad captcha token), body=%s", rec.Code, rec.Body.String())
	}
}

func TestLogin_CaptchaAllowsValidToken(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	h.captcha = &fakeCaptchaVerifier{validTokens: map[string]bool{"good-token": true}}
	email, password, _ := createTestLoginUser(t, h)

	body, _ := json.Marshal(loginReq{Email: email, Password: password, CaptchaToken: "good-token"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (valid captcha token, correct credentials), body=%s", rec.Code, rec.Body.String())
	}
}

func TestLogin_NilCaptchaVerifierSkipsCheckEntirely(t *testing.T) {
	h := newLoginTOTPTestHandler(t) // h.captcha is nil
	email, password, _ := createTestLoginUser(t, h)

	body, _ := json.Marshal(loginReq{Email: email, Password: password}) // no CaptchaToken at all
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nil captcha verifier must not block login), body=%s", rec.Code, rec.Body.String())
	}
}
