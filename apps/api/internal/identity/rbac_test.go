package identity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// testRepo connects to the real dev DB, same skip-if-unconfigured pattern
// as the other integration tests in this repo (e.g.
// internal/integrations/llm/resolve_test.go).
func testRepo(t *testing.T) (*Repo, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio found in DB")
	}
	return NewRepo(pool), pool, studioID
}

// testCipher returns a secrets.Cipher for tests — TokenIssuer now encrypts
// the signed JWT with one (see jwt.go's doc comment), same as production.
func testCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	c, err := secrets.New("MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=") // 32 raw bytes, base64 — test-only key
	if err != nil {
		t.Fatalf("init test cipher: %v", err)
	}
	return c
}

func TestTokenIssuer_PermissionsAndMustResetPasswordSurviveRoundTrip(t *testing.T) {
	issuer := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, 15*time.Minute, testCipher(t))

	studioID := uuid.New()
	u := &User{
		ID: uuid.New(), StudioID: &studioID, Role: RoleStudioStaff, MustResetPassword: true,
	}
	token, exp, jti, err := issuer.Issue(u, []string{"leads", "inbox"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if jti == "" {
		t.Error("Issue returned an empty jti")
	}

	claims, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.ID != jti {
		t.Errorf("claims.ID (jti) = %q, want %q", claims.ID, jti)
	}
	if !claims.MustResetPassword {
		t.Error("MustResetPassword did not survive the round trip")
	}
	if !claims.HasPermission("leads") || !claims.HasPermission("inbox") || claims.HasPermission("payments") {
		t.Errorf("Permissions did not survive the round trip correctly: %v", claims.Permissions)
	}
	if claims.IdleExpiresAt == nil {
		t.Fatal("IdleExpiresAt was not set")
	}
	if !claims.IdleExpiresAt.Time.Before(exp) {
		t.Errorf("IdleExpiresAt (%v) should be before the absolute expiry (%v)", claims.IdleExpiresAt.Time, exp)
	}

	t.Run("Refresh preserves the absolute expiry but mints a new jti and idle deadline", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond) // jwt.NumericDate has 1-second precision
		refreshed, newJTI, err := issuer.Refresh(u, claims.Permissions, claims.ExpiresAt.Time)
		if err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if newJTI == jti {
			t.Error("Refresh should mint a new jti, not reuse the old one")
		}
		newClaims, err := issuer.Parse(refreshed)
		if err != nil {
			t.Fatalf("parse refreshed: %v", err)
		}
		if !newClaims.ExpiresAt.Time.Equal(claims.ExpiresAt.Time) {
			t.Errorf("Refresh changed the absolute expiry: got %v, want %v", newClaims.ExpiresAt.Time, claims.ExpiresAt.Time)
		}
		if !newClaims.IdleExpiresAt.Time.After(claims.IdleExpiresAt.Time) {
			t.Error("Refresh should push the idle deadline forward")
		}
	})

	t.Run("super_admin gets no embedded permissions but HasPermission is always true", func(t *testing.T) {
		super := &User{ID: uuid.New(), Role: RoleSuperAdmin}
		token, _, _, err := issuer.Issue(super, nil)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		claims, err := issuer.Parse(token)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(claims.Permissions) != 0 {
			t.Errorf("expected no embedded permissions for super_admin, got %v", claims.Permissions)
		}
		if !claims.HasPermission("anything") {
			t.Error("super_admin must always have every permission")
		}
	})

	t.Run("issued token is not a bare, decodable JWT on the wire", func(t *testing.T) {
		// Regression guard for the "encrypt the whole token" requirement:
		// a plain JWT is three dot-separated base64url segments. The
		// encrypted envelope must not parse as one.
		parts := 0
		for _, r := range token {
			if r == '.' {
				parts++
			}
		}
		if parts == 2 {
			t.Error("token looks like a bare, unencrypted JWT (two '.' separators)")
		}
	})
}

func TestClaims_PermissionBypassAndCheck(t *testing.T) {
	admin := &Claims{Role: RoleStudioAdmin}
	if !admin.BypassesPermissionChecks() {
		t.Error("studio_admin should bypass permission checks")
	}
	if !admin.HasPermission("anything") {
		t.Error("studio_admin should have every permission")
	}

	super := &Claims{Role: RoleSuperAdmin}
	if !super.BypassesPermissionChecks() || !super.HasPermission("anything") {
		t.Error("super_admin should bypass permission checks and have every permission")
	}

	staff := &Claims{Role: RoleStudioStaff, Permissions: []string{"leads", "inbox"}}
	if staff.BypassesPermissionChecks() {
		t.Error("studio_staff should not bypass permission checks")
	}
	if !staff.HasPermission("leads") || !staff.HasPermission("inbox") {
		t.Error("studio_staff should have its granted permissions")
	}
	if staff.HasPermission("payments") {
		t.Error("studio_staff should not have an ungranted permission")
	}
}

func TestPermissionKeyForPath(t *testing.T) {
	cases := []struct {
		path     string
		wantKeys []string
		wantOK   bool
	}{
		{"/messaging/channels", []string{"channels"}, true},
		{"/messaging/channels/abc-123", []string{"channels"}, true},
		{"/messaging/conversations", []string{"inbox"}, true},
		{"/messaging/stream", []string{"inbox"}, true},                        // SSE feed the Inbox page depends on
		{"/messaging/jobs", []string{"inbox"}, true},                          // pending outbound jobs, shown inside Inbox
		{"/messaging/upload", []string{"inbox", "settings"}, true},            // used by both Inbox and Settings
		{"/messaging/ai/generate", []string{"inbox", "social-planner"}, true}, // used by both Inbox and Social Planner
		{"/messaging/followup-steps", []string{"decision-trees"}, true},       // decision-trees/follow-ups only
		{"/messaging/templates", []string{"inbox"}, true},                     // Inbox's composer only — NOT the "Templates" nav item
		{"/messaging/trigger-links", []string{"inbox"}, true},                 // same
		{"/messaging/leads/cold", []string{"leads"}, true},
		{"/messaging/settings/send-spacing", []string{"settings"}, true},
		{"/messaging/something-unlisted", []string{"settings"}, true}, // falls back to the bare "/messaging" entry
		{"/campaigns", []string{"campaigns"}, true},
		{"/leads/stats", []string{"leads"}, true},
		{"/decision-trees", []string{"decision-trees"}, true},
		{"/knowledge-base/test-chat", []string{"knowledge-base"}, true},
		{"/communication-style", []string{"knowledge-base"}, true}, // corrected: Knowledge Base page, not Settings
		{"/style-refresh-interval", []string{"knowledge-base"}, true},
		{"/program-start-date", []string{"knowledge-base"}, true},
		{"/google-oauth", []string{"channels"}, true}, // corrected: Channels page (Connect Google Ads), not Settings
		{"/ai-models", []string{"settings"}, true},
		{"/totally-unmapped-path", nil, false},
	}
	for _, c := range cases {
		keys, ok := permissionKeyForPath(c.path)
		if ok != c.wantOK || !equalStringSlices(keys, c.wantKeys) {
			t.Errorf("permissionKeyForPath(%q) = (%v, %v), want (%v, %v)", c.path, keys, ok, c.wantKeys, c.wantOK)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestIsExemptPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/users", true},
		{"/users/123", true},
		{"/roles", true},
		{"/roles/123", true},
		{"/leads", false},
		{"/messaging/conversations", false},
	}
	for _, c := range cases {
		if got := isExemptPath(c.path); got != c.want {
			t.Errorf("isExemptPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestRequirePermission_TrustsJWT documents the current, explicitly
// requested design: RequirePermission reads Permissions straight off the
// Claims already attached to the request context — no DB lookup. Two
// requests built from the very same Claims object see the very same
// answer even after the underlying role changes, since nothing here
// re-reads anything; only a freshly issued token (new Claims) would differ.
func TestRequirePermission_TrustsJWT(t *testing.T) {
	studioID := uuid.New()
	newRequest := func(path string, claims *Claims) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/studios/"+studioID.String()+path, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("studioId", studioID.String())
		reqCtx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		reqCtx = WithClaims(reqCtx, claims)
		return req.WithContext(reqCtx)
	}

	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("denied when Claims grants nothing", func(t *testing.T) {
		claims := &Claims{UserID: uuid.New(), StudioID: &studioID, Role: RoleStudioStaff}
		nextCalled = false
		rw := httptest.NewRecorder()
		RequirePermission(next).ServeHTTP(rw, newRequest("/leads", claims))
		if rw.Code != http.StatusForbidden || nextCalled {
			t.Fatalf("got status %d, nextCalled=%v; want 403 and not called", rw.Code, nextCalled)
		}
	})

	t.Run("allowed when Claims already carries the permission", func(t *testing.T) {
		claims := &Claims{UserID: uuid.New(), StudioID: &studioID, Role: RoleStudioStaff, Permissions: []string{"leads"}}
		nextCalled = false
		rw := httptest.NewRecorder()
		RequirePermission(next).ServeHTTP(rw, newRequest("/leads", claims))
		if rw.Code != http.StatusOK || !nextCalled {
			t.Fatalf("got status %d, nextCalled=%v; want 200 and called", rw.Code, nextCalled)
		}
	})

	t.Run("exempt path (/users) allowed even with zero permissions", func(t *testing.T) {
		claims := &Claims{UserID: uuid.New(), StudioID: &studioID, Role: RoleStudioStaff}
		nextCalled = false
		rw := httptest.NewRecorder()
		RequirePermission(next).ServeHTTP(rw, newRequest("/users", claims))
		if rw.Code != http.StatusOK || !nextCalled {
			t.Fatalf("got status %d, nextCalled=%v; /users should be exempt from the permission catalog", rw.Code, nextCalled)
		}
	})

	t.Run("super_admin/studio_admin always allowed regardless of Permissions", func(t *testing.T) {
		for _, role := range []Role{RoleSuperAdmin, RoleStudioAdmin} {
			claims := &Claims{UserID: uuid.New(), StudioID: &studioID, Role: role}
			nextCalled = false
			rw := httptest.NewRecorder()
			RequirePermission(next).ServeHTTP(rw, newRequest("/payments", claims))
			if rw.Code != http.StatusOK || !nextCalled {
				t.Fatalf("role %v: got status %d, nextCalled=%v; want 200 and called", role, rw.Code, nextCalled)
			}
		}
	})
}

func TestRoles_CreateListGetUpdateDelete(t *testing.T) {
	repo, pool, studioID := testRepo(t)
	ctx := context.Background()

	role, err := repo.CreateRole(ctx, studioID, "test-role-"+uuid.NewString(), "a test role", nil)
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	t.Run("duplicate name in the same studio is rejected", func(t *testing.T) {
		if _, err := repo.CreateRole(ctx, studioID, role.Name, "dup", nil); err != ErrRoleNameTaken {
			t.Errorf("got %v, want ErrRoleNameTaken", err)
		}
	})

	t.Run("get scoped to studio", func(t *testing.T) {
		got, err := repo.GetRole(ctx, studioID, role.ID)
		if err != nil {
			t.Fatalf("get role: %v", err)
		}
		if got.Name != role.Name {
			t.Errorf("got name %q, want %q", got.Name, role.Name)
		}
	})

	t.Run("update rejects a collision with another role's name", func(t *testing.T) {
		other, err := repo.CreateRole(ctx, studioID, "other-role-"+uuid.NewString(), "", nil)
		if err != nil {
			t.Fatalf("create other role: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, other.ID) })

		if err := repo.UpdateRole(ctx, studioID, role.ID, other.Name, "", nil); err != ErrRoleNameTaken {
			t.Errorf("got %v, want ErrRoleNameTaken", err)
		}
	})

	t.Run("permission set/get round trip", func(t *testing.T) {
		if err := repo.SetRolePermissions(ctx, role.ID, []string{"leads", "inbox"}, nil); err != nil {
			t.Fatalf("set role permissions: %v", err)
		}
		keys, err := repo.GetRolePermissionKeys(ctx, role.ID)
		if err != nil {
			t.Fatalf("get role permission keys: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("got %d keys, want 2: %v", len(keys), keys)
		}
	})

	t.Run("delete blocked while a user is assigned, succeeds once none are", func(t *testing.T) {
		email := "rbac-test-" + uuid.NewString() + "@example.com"
		userID, err := repo.CreateStudioUser(ctx, studioID, "rbactest_"+uuid.NewString()[:8], "Test", "User", email, role.ID, nil)
		if err != nil {
			t.Fatalf("create studio user: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

		if err := repo.DeleteRole(ctx, studioID, role.ID); err != ErrRoleInUse {
			t.Errorf("got %v, want ErrRoleInUse", err)
		}

		if err := repo.DeactivateStudioUser(ctx, studioID, userID, userID); err != nil {
			t.Fatalf("deactivate user: %v", err)
		}
		// Deactivating doesn't clear role_id, so the role is still "in use"
		// by DeleteRole's count query unless we also clear it — reassign to
		// nothing first, matching the spec's "reassign before deleting" flow.
		if _, err := pool.Exec(ctx, `UPDATE users SET role_id = NULL WHERE id = $1`, userID); err != nil {
			t.Fatalf("clear role_id: %v", err)
		}

		if err := repo.DeleteRole(ctx, studioID, role.ID); err != nil {
			t.Errorf("delete role after reassignment: %v", err)
		}
	})
}

func TestStudioUsers_UsernameUniquePerStudioNotGlobally(t *testing.T) {
	repo, pool, studioA := testRepo(t)
	ctx := context.Background()

	var studioB uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios WHERE id != $1 LIMIT 1`, studioA).Scan(&studioB); err != nil {
		t.Skip("Skipping test; need a second studio in the DB")
	}

	roleA, err := repo.CreateRole(ctx, studioA, "uname-test-role-a-"+uuid.NewString(), "", nil)
	if err != nil {
		t.Fatalf("create role A: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, roleA.ID) })

	roleB, err := repo.CreateRole(ctx, studioB, "uname-test-role-b-"+uuid.NewString(), "", nil)
	if err != nil {
		t.Fatalf("create role B: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, roleB.ID) })

	username := "sameuname_" + uuid.NewString()[:8]

	userA, err := repo.CreateStudioUser(ctx, studioA, username, "A", "One", "uname-a-"+uuid.NewString()+"@example.com", roleA.ID, nil)
	if err != nil {
		t.Fatalf("create user in studio A: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userA) })

	// Same username, different studio: must succeed.
	userB, err := repo.CreateStudioUser(ctx, studioB, username, "B", "Two", "uname-b-"+uuid.NewString()+"@example.com", roleB.ID, nil)
	if err != nil {
		t.Fatalf("expected same username in a different studio to succeed, got: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userB) })

	// Same username, same studio: must be rejected.
	if _, err := repo.CreateStudioUser(ctx, studioA, username, "A", "Dup", "uname-dup-"+uuid.NewString()+"@example.com", roleA.ID, nil); err != ErrUsernameTaken {
		t.Errorf("got %v, want ErrUsernameTaken", err)
	}
}

func TestStudioUsers_DeactivatedUserCannotBeFoundByEmail(t *testing.T) {
	repo, pool, studioID := testRepo(t)
	ctx := context.Background()

	role, err := repo.CreateRole(ctx, studioID, "deactivate-test-role-"+uuid.NewString(), "", nil)
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	email := "deactivate-test-" + uuid.NewString() + "@example.com"
	userID, err := repo.CreateStudioUser(ctx, studioID, "deactuser_"+uuid.NewString()[:8], "D", "User", email, role.ID, nil)
	if err != nil {
		t.Fatalf("create studio user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	if _, err := repo.FindByEmail(ctx, email); err != nil {
		t.Fatalf("expected to find the user before deactivation, got: %v", err)
	}

	if err := repo.DeactivateStudioUser(ctx, studioID, userID, userID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if _, err := repo.FindByEmail(ctx, email); err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound — a deactivated user must not be able to log in", err)
	}
}
