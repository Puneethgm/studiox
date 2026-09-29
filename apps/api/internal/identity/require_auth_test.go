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
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, idleTTL, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, err := tokens.Issue(u, nil)
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

// TestRequireAuth_RejectsRevokedSession confirms a session killed via
// SessionStore.Revoke (what logout/password-change do) is rejected
// immediately, even though its IdleExpiresAt hasn't passed — revocation is
// the one thing the token can't self-enforce, hence it still needs Redis.
func TestRequireAuth_RejectsRevokedSession(t *testing.T) {
	sessions := testSessionStore(t, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Minute, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, jti, err := tokens.Issue(u, nil)
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

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
