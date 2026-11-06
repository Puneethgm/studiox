package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/rediscache"
)

// TestRequireAuth_FailsClosedWhenRevocationStoreUnreachable covers
// platform-security-hardening task 1.10: if the revocation check itself
// can't complete (Redis unreachable), the request must be rejected, not
// treated as "not revoked" and let through.
func TestRequireAuth_FailsClosedWhenRevocationStoreUnreachable(t *testing.T) {
	// An address nothing listens on — rediscache.New is lazy, so this
	// only fails once a command is actually attempted, exactly like a
	// real Redis outage.
	unreachable := rediscache.New("127.0.0.1:1", "wrong-password-but-irrelevant-connection-never-succeeds", 0)
	sessions := NewSessionStore(unreachable, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	h := &Handler{tokens: tokens, cookie: config.CookieConfig{Name: "px_session"}, sessions: sessions}

	u := &User{ID: uuid.New(), Role: RoleSuperAdmin}
	token, _, _, _, _, err := tokens.Issue(context.Background(), u, nil, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// Deliberately skip sessions.Create — the point is that Exists/
	// IsRevoked can't even be answered, not that the session is genuinely
	// absent; a real outage looks the same from RequireAuth's side either
	// way.

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := h.RequireAuth(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "px_session", Value: token})
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("expected the request to be rejected when the revocation store is unreachable, got 200 OK")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (fail-closed on a store error, not a user-facing 401)", rec.Code)
	}
}
