package studios

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
)

func claimsReq(method, path string, role identity.Role, studioID *uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	c := &identity.Claims{UserID: uuid.New(), Role: role, StudioID: studioID}
	return req.WithContext(identity.WithClaims(req.Context(), c))
}

// A studio-bound user must not reach the platform-wide "global" scope, whether it
// appears as a literal path segment or as the {id} param. A super-admin still can.
func TestRequireActiveStudioBlocksGlobalScopeForStudioUsers(t *testing.T) {
	h := &Handler{} // no svc: the global check must reject before any DB access
	sid := uuid.New()

	paths := []string{
		"/api/v1/me/studios/global/plans",
		"/api/v1/me/studios/global/payments/stripe",
		"/api/v1/me/studios/global/billing/history",
		"/api/v1/studios/global/social-posts",
	}
	for _, role := range []identity.Role{identity.RoleStudioAdmin, identity.RoleStudioStaff} {
		for _, p := range paths {
			called := false
			mw := h.RequireActiveStudio(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
			rr := httptest.NewRecorder()
			mw.ServeHTTP(rr, claimsReq(http.MethodPut, p, role, &sid))
			if rr.Code != http.StatusForbidden || called {
				t.Errorf("role=%s path=%s: code=%d called=%v, want 403 and not called", role, p, rr.Code, called)
			}
		}
	}

	called := false
	mw := h.RequireActiveStudio(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, claimsReq(http.MethodPut, "/api/v1/me/studios/global/plans", identity.RoleSuperAdmin, nil))
	if !called {
		t.Errorf("super_admin must still reach global scope, got code=%d", rr.Code)
	}
}

func TestIsGlobalScope(t *testing.T) {
	cases := map[string]bool{
		"/api/v1/me/studios/global/plans":                   true,
		"/api/v1/studios/global/leads":                      true,
		"/api/v1/me/studios/" + uuid.NewString() + "/plans": false,
		"/api/v1/studios/" + uuid.NewString() + "/global":   false, // "global" not directly after "studios"
	}
	for path, want := range cases {
		if got := isGlobalScope(httptest.NewRequest(http.MethodGet, path, nil)); got != want {
			t.Errorf("isGlobalScope(%q) = %v, want %v", path, got, want)
		}
	}
}

// Only the studio's own admin may delete the studio; staff of that studio may not,
// even though they belong to it. The role check runs before any lookup.
func TestDeleteAccountRequiresStudioAdmin(t *testing.T) {
	h := &Handler{}
	sid := uuid.New()
	r := chi.NewRouter()
	r.Delete("/me/studios/{id}/delete-account", h.deleteAccount)

	req := claimsReq(http.MethodDelete, "/me/studios/"+sid.String()+"/delete-account", identity.RoleStudioStaff, &sid)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("studio_staff delete: code=%d, want 403 (body %s)", rr.Code, rr.Body.String())
	}
}
