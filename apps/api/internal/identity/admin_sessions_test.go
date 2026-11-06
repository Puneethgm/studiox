package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TestForceLogoutStudioUser_RejectsUserOutsideStudio and
// TestForceLogoutStudioUser_EndsSessionsForMember cover platform-
// security-hardening task 1.6's studio-scoped force-logout.
func TestForceLogoutStudioUser_RejectsUserOutsideStudio(t *testing.T) {
	h, _ := newSessionHTTPTestHandler(t)
	ctx := context.Background()

	var studioID uuid.UUID
	if err := h.repo.pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping; no studio found in DB")
	}
	randomUserID := uuid.New() // definitely not a member of studioID

	req := httptest.NewRequest(http.MethodPost, "/studios/"+studioID.String()+"/users/"+randomUserID.String()+"/sessions/end", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studioId", studioID.String())
	rctx.URLParams.Add("userId", randomUserID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.forceLogoutStudioUser(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
}

func TestForceLogoutStudioUser_EndsSessionsForMember(t *testing.T) {
	h, _ := newSessionHTTPTestHandler(t)
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
	memberID, err := h.repo.CreateStudioUser(ctx, studioID, "force-logout-member", "Test", "User",
		"force-logout-member-"+uuid.New().String()+"@example.com", role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, memberID) })

	sessionID := uuid.New()
	if err := h.repo.CreateSession(ctx, sessionID, memberID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	req := httptest.NewRequest(http.MethodPost, "/studios/"+studioID.String()+"/users/"+memberID.String()+"/sessions/end", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studioId", studioID.String())
	rctx.URLParams.Add("userId", memberID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.forceLogoutStudioUser(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	got, err := h.repo.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.IsActive() {
		t.Error("expected the member's session to be revoked")
	}
}

// TestAdminForceLogoutUser_EndsSessions and
// TestAdminEndAllSessions_EndsEverySession cover the super_admin,
// platform-wide endpoints.
func TestAdminForceLogoutUser_EndsSessions(t *testing.T) {
	h, userID := newSessionHTTPTestHandler(t)
	ctx := context.Background()

	sessionID := uuid.New()
	if err := h.repo.CreateSession(ctx, sessionID, userID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+userID.String()+"/sessions/end", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userId", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.adminForceLogoutUser(rec, req)
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
}

// TestAdminEndAllSessions_EndsEverySession is genuinely platform-wide —
// RevokeAllSessions has no narrower scope by design — so against the
// shared local dev DB it would also revoke anyone's real, currently
// logged-in browser session. Opt in explicitly rather than running this
// automatically alongside the rest of the suite.
func TestAdminEndAllSessions_EndsEverySession(t *testing.T) {
	if os.Getenv("ALLOW_DESTRUCTIVE_SESSION_TESTS") == "" {
		t.Skip("skipping: platform-wide RevokeAllSessions would also log out any real session on this DB; set ALLOW_DESTRUCTIVE_SESSION_TESTS=1 to run it deliberately")
	}
	h, userID := newSessionHTTPTestHandler(t)
	ctx := context.Background()

	id1, id2 := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{id1, id2} {
		if err := h.repo.CreateSession(ctx, id, userID, "", "", "", "", false); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, []uuid.UUID{id1, id2}) })

	req := httptest.NewRequest(http.MethodPost, "/admin/sessions/end-all", nil)
	rec := httptest.NewRecorder()
	h.adminEndAllSessions(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	for _, id := range []uuid.UUID{id1, id2} {
		got, err := h.repo.GetSession(ctx, id)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.IsActive() {
			t.Errorf("session %v should have been ended by end-all", id)
		}
		revoked, err := h.sessions.IsRevoked(ctx, id)
		if err != nil {
			t.Fatalf("IsRevoked: %v", err)
		}
		if !revoked {
			t.Errorf("session %v should report revoked via the fast Redis check too", id)
		}
	}
}
