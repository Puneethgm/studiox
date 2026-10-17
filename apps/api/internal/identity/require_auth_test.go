package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/config"
)

// TestRequireAuth_RotatesAndEnforcesIdleTimeout exercises the actual
// middleware end-to-end (not just its pieces in isolation): a request
// within the idle window succeeds and rotates the cookie (new jti, pushed-
// forward idle deadline, same absolute expiry); a request presenting that
// rotated cookie after the idle window has elapsed is rejected, proving
// the idle timeout is enforced by the token's own IdleExpiresAt claim, not
// by waiting out Redis's TTL.
func TestRequireAuth_RotatesAndEnforcesIdleTimeout(t *testing.T) {
	idleTTL := 2 * time.Second
	sessions := testSessionStore(t, idleTTL)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, idleTTL, 24*time.Hour, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, _, err := tokens.Issue(u, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := sessions.Create(context.Background(), jti, u.ID); err != nil {
		t.Fatalf("sessions.Create: %v", err)
	}

	var sawClaims *Claims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawClaims, _ = ClaimsFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := h.RequireAuth(next)

	// Request #1: within the idle window — should succeed and rotate.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.AddCookie(&http.Cookie{Name: "px_session", Value: token})
	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request #1 status = %d, want 200", rec1.Code)
	}
	if sawClaims == nil || sawClaims.UserID != u.ID {
		t.Fatal("RequireAuth did not inject claims into the request context")
	}
	rotated := findCookie(rec1.Result().Cookies(), "px_session")
	if rotated == nil {
		t.Fatal("request #1 did not set a rotated session cookie")
	}
	if rotated.Value == token {
		t.Error("rotated cookie has the same value as the original — rotation did not happen")
	}

	// Request #2, immediately, using the rotated cookie — still within the
	// idle window, should also succeed (proves the rotated cookie is
	// itself usable, not a one-shot).
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "px_session", Value: rotated.Value})
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("request #2 (rotated cookie, still fresh) status = %d, want 200", rec2.Code)
	}
	rotatedAgain := findCookie(rec2.Result().Cookies(), "px_session")
	if rotatedAgain == nil || rotatedAgain.Value == rotated.Value {
		t.Fatal("request #2 did not rotate the cookie again")
	}

	// Wait past the idle window, then present the latest cookie again.
	time.Sleep(idleTTL + 300*time.Millisecond)
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.AddCookie(&http.Cookie{Name: "px_session", Value: rotatedAgain.Value})
	rec3 := httptest.NewRecorder()
	mw.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("request #3 (after idle window elapsed) status = %d, want 401", rec3.Code)
	}
}

// TestRequireAuth_EnforcesHardBackstopDespiteContinuousActivity proves the
// one thing sliding activity must NOT be able to do: outrun
// Claims.SessionMaxExpiresAt. idleTTL is deliberately huge (an hour) so
// only the hard backstop (2s here) is in play — two back-to-back requests,
// each rotating the session like real continuous use, still get rejected
// once the backstop passes.
func TestRequireAuth_EnforcesHardBackstopDespiteContinuousActivity(t *testing.T) {
	maxAge := 2 * time.Second
	sessions := testSessionStore(t, time.Hour)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, maxAge, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, _, err := tokens.Issue(u, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := sessions.Create(context.Background(), jti, u.ID); err != nil {
		t.Fatalf("sessions.Create: %v", err)
	}

	mw := h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request #1, immediately — well within every window, succeeds and
	// rotates (simulating real activity, not a stale cookie).
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.AddCookie(&http.Cookie{Name: "px_session", Value: token})
	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request #1 status = %d, want 200", rec1.Code)
	}
	rotated := findCookie(rec1.Result().Cookies(), "px_session")
	if rotated == nil {
		t.Fatal("request #1 did not set a rotated session cookie")
	}

	// Wait past the hard backstop, then present the rotated (still "fresh"
	// by idle standards) cookie — must be rejected despite the simulated
	// activity, proving the backstop is a real ceiling, not just advisory.
	time.Sleep(maxAge + 300*time.Millisecond)
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "px_session", Value: rotated.Value})
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("request #2 (after hard backstop elapsed, despite activity) status = %d, want 401", rec2.Code)
	}
}

// TestRequireAuth_RejectsRevokedSession confirms a session killed via
// SessionStore.Revoke (what logout/password-change do) is rejected
// immediately, even though its IdleExpiresAt hasn't passed — revocation is
// the one thing the token can't self-enforce, hence it still needs Redis.
func TestRequireAuth_RejectsRevokedSession(t *testing.T) {
	sessions := testSessionStore(t, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Minute, 24*time.Hour, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, _, err := tokens.Issue(u, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := sessions.Create(context.Background(), jti, u.ID); err != nil {
		t.Fatalf("sessions.Create: %v", err)
	}

	mw := h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Simulate logout.
	if err := sessions.Revoke(context.Background(), jti); err != nil {
		t.Fatalf("sessions.Revoke: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "px_session", Value: token})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status after revoke = %d, want 401", rec.Code)
	}
}

// TestRequireAuth_CSRF proves the CSRF check actually gates mutating
// requests (and nothing else): a POST with no X-CSRF-Token header is
// rejected, a POST with the wrong value is rejected, a POST with the
// correct value (from claims.CSRFToken) succeeds, and a GET with no header
// at all still succeeds — CSRF only applies to state-changing methods.
func TestRequireAuth_CSRF(t *testing.T) {
	sessions := testSessionStore(t, time.Hour)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, csrfToken, err := tokens.Issue(u, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := sessions.Create(context.Background(), jti, u.ID); err != nil {
		t.Fatalf("sessions.Create: %v", err)
	}

	mw := h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	newReq := func(method, csrfHeader string) *http.Request {
		req := httptest.NewRequest(method, "/", nil)
		req.AddCookie(&http.Cookie{Name: "px_session", Value: token})
		if csrfHeader != "" {
			req.Header.Set("X-CSRF-Token", csrfHeader)
		}
		return req
	}

	t.Run("POST with no X-CSRF-Token header is rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, newReq(http.MethodPost, ""))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("POST with the wrong CSRF token is rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, newReq(http.MethodPost, "not-the-real-token"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("POST with the correct CSRF token succeeds", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, newReq(http.MethodPost, csrfToken))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if findCookie(rec.Result().Cookies(), csrfCookieName) == nil {
			t.Error("response did not re-set the px_csrf cookie")
		}
	})

	t.Run("GET with no CSRF header at all still succeeds — safe methods are exempt", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, newReq(http.MethodGet, ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("rollout grace: a pre-existing session with no CSRFToken claim at all self-heals instead of locking the admin out", func(t *testing.T) {
		legacyUser := &User{ID: uuid.New(), Role: RoleSuperAdmin}
		// Bypasses Issue (which always sets a real token now) to simulate a
		// session minted by the pre-CSRF code — the only way claims.CSRFToken
		// is ever "".
		legacyToken, _, legacyJTI, err := tokens.issue(legacyUser, nil, time.Now().Add(time.Hour), time.Now().Add(24*time.Hour), "")
		if err != nil {
			t.Fatalf("issue legacy token: %v", err)
		}
		if err := sessions.Create(context.Background(), legacyJTI, legacyUser.ID); err != nil {
			t.Fatalf("sessions.Create: %v", err)
		}

		// First mutating request: no token was ever issued to present, so it
		// can't be checked — let through rather than locking this admin out
		// for the rest of their session.
		req1 := httptest.NewRequest(http.MethodPost, "/", nil)
		req1.AddCookie(&http.Cookie{Name: "px_session", Value: legacyToken})
		rec1 := httptest.NewRecorder()
		mw.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Fatalf("first mutating request on a legacy session: status = %d, want 200", rec1.Code)
		}
		newCSRFCookie := findCookie(rec1.Result().Cookies(), csrfCookieName)
		if newCSRFCookie == nil || newCSRFCookie.Value == "" {
			t.Fatal("first request did not mint and set a real CSRF token")
		}
		rotatedSession := findCookie(rec1.Result().Cookies(), "px_session")
		if rotatedSession == nil {
			t.Fatal("first request did not rotate the session cookie")
		}

		// Second mutating request, using the rotated session + the newly
		// minted CSRF token: now properly enforced, same as any other
		// session from here on.
		req2 := httptest.NewRequest(http.MethodPost, "/", nil)
		req2.AddCookie(&http.Cookie{Name: "px_session", Value: rotatedSession.Value})
		req2.Header.Set("X-CSRF-Token", newCSRFCookie.Value)
		rec2 := httptest.NewRecorder()
		mw.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("second mutating request with the newly minted token: status = %d, want 200", rec2.Code)
		}

		// A forged request (correct session cookie, wrong/no CSRF token) is
		// now properly rejected — the grace window is over for this session.
		req3 := httptest.NewRequest(http.MethodPost, "/", nil)
		req3.AddCookie(&http.Cookie{Name: "px_session", Value: rotatedSession.Value})
		rec3 := httptest.NewRecorder()
		mw.ServeHTTP(rec3, req3)
		if rec3.Code != http.StatusForbidden {
			t.Fatalf("third mutating request with no token (post-grace): status = %d, want 403", rec3.Code)
		}
	})
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
