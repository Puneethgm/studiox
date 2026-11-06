package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// newSessionHTTPTestHandler builds a Handler wired to the real local DB
// and Redis for the self-service session endpoints (platform-security-
// hardening task 1.5).
func newSessionHTTPTestHandler(t *testing.T) (*Handler, uuid.UUID) {
	t.Helper()
	repo, _, _ := testRepo(t)
	sessions := testSessionStore(t, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	h := &Handler{repo: repo, sessions: sessions, tokens: tokens}
	userID := testUserID(t, repo)
	return h, userID
}

// withClaims injects claims into a request's context the same way
// RequireAuth does, letting these tests exercise the handlers directly
// without going through the full middleware chain (which is already
// covered by require_auth_test.go).
func withClaims(r *http.Request, c *Claims) *http.Request {
	return r.WithContext(WithClaims(r.Context(), c))
}

func TestListMySessions_FlagsCurrentAndExcludesRevoked(t *testing.T) {
	h, userID := newSessionHTTPTestHandler(t)
	ctx := context.Background()

	currentID, otherID, revokedID := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{currentID, otherID, revokedID} {
		if err := h.repo.CreateSession(ctx, id, userID, "", "", "", "", false); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, []uuid.UUID{currentID, otherID, revokedID})
	})
	if err := h.repo.RevokeSession(ctx, revokedID, "test setup"); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	req := withClaims(httptest.NewRequest(http.MethodGet, "/auth/sessions", nil), &Claims{UserID: userID, SessionID: currentID})
	rec := httptest.NewRecorder()
	h.listMySessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var got []sessionRes
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, s := range got {
		seen[s.ID] = true
		if s.ID == currentID && !s.IsCurrent {
			t.Error("the session serving this request must be flagged IsCurrent")
		}
		if s.ID == otherID && s.IsCurrent {
			t.Error("a different session must not be flagged IsCurrent")
		}
	}
	if !seen[currentID] || !seen[otherID] {
		t.Errorf("expected both active sessions in the list, got %+v", got)
	}
	if seen[revokedID] {
		t.Error("a revoked session must not appear in the list")
	}
}

func TestEndMySession_RejectsEndingAnotherUsersSession(t *testing.T) {
	h, userID := newSessionHTTPTestHandler(t)
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
	otherUserID, err := h.repo.CreateStudioUser(ctx, studioID, "other-user-test", "Other", "User",
		"other-user-test-"+uuid.New().String()+"@example.com", role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUserID) })

	theirSessionID := uuid.New()
	if err := h.repo.CreateSession(ctx, theirSessionID, otherUserID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, theirSessionID) })

	req := httptest.NewRequest(http.MethodDelete, "/auth/sessions/"+theirSessionID.String(), nil)
	req = withClaims(req, &Claims{UserID: userID, SessionID: uuid.New()})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", theirSessionID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.endMySession(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}

	got, err := h.repo.GetSession(ctx, theirSessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if !got.IsActive() {
		t.Error("the other user's session must not have been revoked")
	}
}

func TestEndMySession_EndsOwnSession(t *testing.T) {
	h, userID := newSessionHTTPTestHandler(t)
	ctx := context.Background()
	sessionID := uuid.New()
	if err := h.repo.CreateSession(ctx, sessionID, userID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	req := httptest.NewRequest(http.MethodDelete, "/auth/sessions/"+sessionID.String(), nil)
	req = withClaims(req, &Claims{UserID: userID, SessionID: uuid.New()})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", sessionID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.endMySession(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	got, err := h.repo.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.IsActive() {
		t.Error("expected the session to be revoked")
	}
	revoked, err := h.sessions.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Error("expected the fast Redis revocation check to also reflect the end")
	}
}

func TestEndOtherMySessions_LeavesCurrentActive(t *testing.T) {
	h, userID := newSessionHTTPTestHandler(t)
	ctx := context.Background()

	currentID, otherA, otherB := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{currentID, otherA, otherB} {
		if err := h.repo.CreateSession(ctx, id, userID, "", "", "", "", false); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, []uuid.UUID{currentID, otherA, otherB})
	})

	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/sessions/end-others", nil), &Claims{UserID: userID, SessionID: currentID})
	rec := httptest.NewRecorder()
	h.endOtherMySessions(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	current, err := h.repo.GetSession(ctx, currentID)
	if err != nil {
		t.Fatalf("GetSession(current): %v", err)
	}
	if !current.IsActive() {
		t.Error("the current session must remain active")
	}
	for _, id := range []uuid.UUID{otherA, otherB} {
		got, err := h.repo.GetSession(ctx, id)
		if err != nil {
			t.Fatalf("GetSession(%v): %v", id, err)
		}
		if got.IsActive() {
			t.Errorf("session %v should have been ended by end-others", id)
		}
	}
}
