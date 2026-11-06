package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/projectx/api/internal/platform/config"
)

// createTestLoginUser creates a real studio_staff user with a known
// plaintext password, so these tests can drive the real login handler
// end to end rather than constructing claims by hand.
func createTestLoginUser(t *testing.T, h *Handler) (email, password string, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var studioID uuid.UUID
	if err := h.repo.pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping; no studio found in DB")
	}
	role, err := h.repo.CreateRole(ctx, studioID, "test-role-"+uuid.New().String(), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	email = fmt.Sprintf("login-totp-test-%s@example.com", uuid.New())
	password = "a-known-test-password-123"
	userID, err = h.repo.CreateStudioUserWithPassword(ctx, studioID, "login-totp-test", "Test", "User", email, role.ID, nil, password)
	if err != nil {
		t.Fatalf("CreateStudioUserWithPassword: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })
	return email, password, userID
}

// newLoginTOTPTestHandler builds a full Handler (DB + Redis + cipher)
// for the two-step login flow (platform-security-hardening task 2.3).
func newLoginTOTPTestHandler(t *testing.T) *Handler {
	t.Helper()
	repo, _, _ := testRepo(t)
	sessions := testSessionStore(t, time.Hour)
	pendingAuth := testPendingAuthStore(t)
	lockout := testTwoFALockoutStore(t)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	return &Handler{
		repo: repo, sessions: sessions, tokens: tokens, cipher: testCipher(t),
		pendingAuth: pendingAuth, totpLockout: lockout, cookie: config.CookieConfig{Name: "px_session"},
	}
}

// enrollTOTPForTest completes enrollment for userID the same way the
// real setup+confirm endpoints do, returning the plaintext secret so
// the test can generate real codes.
func enrollTOTPForTest(t *testing.T, h *Handler, userID uuid.UUID) string {
	t.Helper()
	ctx := context.Background()
	secret, _, err := generateTOTPSecret("test@example.com")
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	ciphertext, err := h.cipher.Encrypt(secret)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if err := h.repo.SaveTOTPSecretPending(ctx, userID, ciphertext); err != nil {
		t.Fatalf("SaveTOTPSecretPending: %v", err)
	}
	if err := h.repo.EnableTOTP(ctx, userID); err != nil {
		t.Fatalf("EnableTOTP: %v", err)
	}
	return secret
}

func TestLoginThenVerifyTOTP_FullFlow(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()

	email, password, userID := createTestLoginUser(t, h)
	secret := enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	})

	// Step 1: password login returns twoFactorRequired, no session cookie.
	loginBody, _ := json.Marshal(loginReq{Email: email, Password: password})
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(loginBody)))
	loginReqHTTP.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.login(loginRec, loginReqHTTP)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body=%s", loginRec.Code, loginRec.Body.String())
	}
	if len(loginRec.Result().Cookies()) != 0 {
		t.Fatal("expected no session cookie to be set when TOTP is required")
	}
	var twoFARes twoFARequiredRes
	if err := json.Unmarshal(loginRec.Body.Bytes(), &twoFARes); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if !twoFARes.TwoFactorRequired || twoFARes.PendingToken == "" {
		t.Fatalf("unexpected login response: %+v", twoFARes)
	}

	// Step 2, wrong code: rejected, pending token still usable afterward.
	wrongBody, _ := json.Marshal(verifyLoginTOTPReq{PendingToken: twoFARes.PendingToken, Code: "000000"})
	wrongReq := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(wrongBody)))
	wrongReq.Header.Set("Content-Type", "application/json")
	wrongRec := httptest.NewRecorder()
	h.verifyLoginTOTP(wrongRec, wrongReq)
	realCode, _ := totp.GenerateCode(secret, time.Now())
	if wrongRec.Code != http.StatusUnprocessableEntity && "000000" != realCode {
		t.Fatalf("verify (wrong code) status = %d, want 422, body=%s", wrongRec.Code, wrongRec.Body.String())
	}

	// Step 2, correct code: succeeds, issues a session.
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	correctBody, _ := json.Marshal(verifyLoginTOTPReq{PendingToken: twoFARes.PendingToken, Code: code})
	correctReq := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(correctBody)))
	correctReq.Header.Set("Content-Type", "application/json")
	correctRec := httptest.NewRecorder()
	h.verifyLoginTOTP(correctRec, correctReq)
	if correctRec.Code != http.StatusOK {
		t.Fatalf("verify (correct code) status = %d, want 200, body=%s", correctRec.Code, correctRec.Body.String())
	}
	if len(correctRec.Result().Cookies()) == 0 {
		t.Fatal("expected a session cookie to be set after a correct code")
	}

	// The pending token is now consumed — reusing it fails even with the
	// same correct code (also exercises replay rejection, since the TOTP
	// step itself was already marked used).
	reuseReq := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(correctBody)))
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseRec := httptest.NewRecorder()
	h.verifyLoginTOTP(reuseRec, reuseReq)
	if reuseRec.Code == http.StatusOK {
		t.Error("expected reusing a consumed pending token to fail")
	}
}

// TestVerifyLoginTOTP_LocksOutAfterFiveWrongCodes covers platform-
// security-hardening task 2.4 through the real HTTP handler: 5 wrong
// codes lock the account, and a 6th attempt with the CORRECT code is
// still rejected until the lockout window ends.
func TestVerifyLoginTOTP_LocksOutAfterFiveWrongCodes(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)
	secret := enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_ = h.totpLockout.Reset(ctx, userID)
	})

	pendingToken, err := h.pendingAuth.Create(ctx, userID)
	if err != nil {
		t.Fatalf("pendingAuth.Create: %v", err)
	}

	attempt := func(code string) int {
		body, _ := json.Marshal(verifyLoginTOTPReq{PendingToken: pendingToken, Code: code})
		req := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.verifyLoginTOTP(rec, req)
		return rec.Code
	}

	for i := 0; i < totpLockoutThreshold; i++ {
		status := attempt("000000")
		if status == http.StatusOK {
			t.Fatalf("attempt %d with a wrong code unexpectedly succeeded", i+1)
		}
	}

	locked, err := h.totpLockout.IsLocked(ctx, userID)
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if !locked {
		t.Fatal("expected the account to be locked after 5 wrong codes")
	}

	// Re-issue a fresh pending token (the old one is still valid, but
	// using a new one isolates this assertion from Step 2's token
	// lifecycle) and try the REAL code — must still be rejected while
	// locked.
	freshToken, err := h.pendingAuth.Create(ctx, userID)
	if err != nil {
		t.Fatalf("pendingAuth.Create (fresh): %v", err)
	}
	realCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	body, _ := json.Marshal(verifyLoginTOTPReq{PendingToken: freshToken, Code: realCode})
	req := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.verifyLoginTOTP(rec, req)
	if rec.Code != http.StatusLocked {
		t.Fatalf("status with the correct code while locked = %d, want 423, body=%s", rec.Code, rec.Body.String())
	}
}

// TestVerifyLoginRecoveryCode_SignsInAndMarksCodeUsed covers platform-
// security-hardening task 2.7: the lost-phone path.
func TestVerifyLoginRecoveryCode_SignsInAndMarksCodeUsed(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)
	enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID)
	})

	codes, err := generateRecoveryCodes()
	if err != nil {
		t.Fatalf("generateRecoveryCodes: %v", err)
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		h2, err := HashPassword(normalizeRecoveryCode(c))
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		hashes[i] = h2
	}
	if err := h.repo.CreateRecoveryCodes(ctx, userID, hashes); err != nil {
		t.Fatalf("CreateRecoveryCodes: %v", err)
	}

	pendingToken, err := h.pendingAuth.Create(ctx, userID)
	if err != nil {
		t.Fatalf("pendingAuth.Create: %v", err)
	}
	body, _ := json.Marshal(verifyLoginRecoveryCodeReq{PendingToken: pendingToken, Code: codes[0]})
	req := httptest.NewRequest(http.MethodPost, "/auth/login/recovery-code", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.verifyLoginRecoveryCode(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected a session cookie to be set after a valid recovery code")
	}

	count, err := h.repo.CountUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes: %v", err)
	}
	if count != len(codes)-1 {
		t.Errorf("CountUnusedRecoveryCodes = %d, want %d (one consumed)", count, len(codes)-1)
	}

	// Reusing the same code (even with a fresh pending token) fails.
	freshToken, _ := h.pendingAuth.Create(ctx, userID)
	reuseBody, _ := json.Marshal(verifyLoginRecoveryCodeReq{PendingToken: freshToken, Code: codes[0]})
	reuseReq := httptest.NewRequest(http.MethodPost, "/auth/login/recovery-code", strings.NewReader(string(reuseBody)))
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseRec := httptest.NewRecorder()
	h.verifyLoginRecoveryCode(reuseRec, reuseReq)
	if reuseRec.Code == http.StatusOK {
		t.Error("expected reusing an already-consumed recovery code to fail")
	}
}

// TestRememberBrowser_SkipsCodeStepOnNextLogin covers platform-security-
// hardening task 2.5 end to end through the real login/verify handlers.
func TestRememberBrowser_SkipsCodeStepOnNextLogin(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	email, password, userID := createTestLoginUser(t, h)
	secret := enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID)
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM remember_browser_tokens WHERE user_id = $1`, userID)
	})

	// Step 1+2: log in and complete the code step with rememberBrowser=true.
	pendingToken, err := h.pendingAuth.Create(ctx, userID)
	if err != nil {
		t.Fatalf("pendingAuth.Create: %v", err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode: %v", err)
	}
	verifyBody, _ := json.Marshal(verifyLoginTOTPReq{PendingToken: pendingToken, Code: code, RememberBrowser: true})
	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/login/2fa", strings.NewReader(string(verifyBody)))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyRec := httptest.NewRecorder()
	h.verifyLoginTOTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200, body=%s", verifyRec.Code, verifyRec.Body.String())
	}
	var rememberCookie *http.Cookie
	for _, c := range verifyRec.Result().Cookies() {
		if c.Name == rememberBrowserCookieName {
			rememberCookie = c
		}
	}
	if rememberCookie == nil {
		t.Fatal("expected a remember-browser cookie to be set")
	}

	// Next login, presenting that cookie: skips the code step entirely.
	loginBody, _ := json.Marshal(loginReq{Email: email, Password: password})
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(loginBody)))
	loginReqHTTP.Header.Set("Content-Type", "application/json")
	loginReqHTTP.AddCookie(rememberCookie)
	loginRec := httptest.NewRecorder()
	h.login(loginRec, loginReqHTTP)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login (with remember cookie) status = %d, want 200, body=%s", loginRec.Code, loginRec.Body.String())
	}
	var meResult map[string]any
	if err := json.Unmarshal(loginRec.Body.Bytes(), &meResult); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if _, isTwoFA := meResult["twoFactorRequired"]; isTwoFA {
		t.Fatal("expected the remembered browser to skip the code step, got twoFactorRequired response")
	}
	if len(loginRec.Result().Cookies()) == 0 {
		t.Fatal("expected a session cookie to be issued directly")
	}
}

// TestRememberBrowser_NeverGrantedForSuperAdmin covers the "never
// super_admin, regardless of the request" rule.
func TestRememberBrowser_NeverGrantedForSuperAdmin(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	u := &User{ID: uuid.New(), Role: RoleSuperAdmin, Email: "super@example.com"}
	rec := httptest.NewRecorder()
	h.maybeSetRememberBrowserCookie(rec, httptest.NewRequest(http.MethodPost, "/x", nil), u, true)
	if len(rec.Result().Cookies()) != 0 {
		t.Error("expected no remember-browser cookie to ever be set for super_admin, even when requested")
	}
}

func TestLogin_NoTOTPEnrolled_LogsInDirectly(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	email, password, userID := createTestLoginUser(t, h)
	_ = userID

	loginBody, _ := json.Marshal(loginReq{Email: email, Password: password})
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(loginBody)))
	loginReqHTTP.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.login(loginRec, loginReqHTTP)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body=%s", loginRec.Code, loginRec.Body.String())
	}
	if len(loginRec.Result().Cookies()) == 0 {
		t.Fatal("expected a session cookie to be set immediately when TOTP isn't enrolled")
	}
}
