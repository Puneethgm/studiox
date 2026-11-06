package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequireTOTPEnrollment_EnrolledUserPassesThrough(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)
	enrollTOTPForTest(t, h, userID)
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID) })

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := h.RequireTOTPEnrollment(next)
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), &Claims{UserID: userID, Role: RoleStudioStaff})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireTOTPEnrollment_UnenrolledSuperAdminBlockedImmediately(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	_, _, userID := createTestLoginUser(t, h) // not enrolled

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := h.RequireTOTPEnrollment(next)
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), &Claims{UserID: userID, Role: RoleSuperAdmin})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (no grace for super_admin), body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireTOTPEnrollment_UnenrolledStudioAdminBlockedImmediately(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	_, _, userID := createTestLoginUser(t, h) // not enrolled

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := h.RequireTOTPEnrollment(next)
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), &Claims{UserID: userID, Role: RoleStudioAdmin})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (no grace for studio_admin), body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireTOTPEnrollment_UnenrolledStaffWithinGracePasses(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	_, _, userID := createTestLoginUser(t, h) // CreatedAt is "now" — well within any grace window

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := h.RequireTOTPEnrollment(next)
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), &Claims{UserID: userID, Role: RoleStudioStaff})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (still within grace), body=%s", rec.Code, rec.Body.String())
	}
}

// TestRequireTOTPEnrollment_StaffPastGraceIsBlocked exercises the
// "14-day grace then enforced" boundary. totpEnforcementAnnouncedAt
// anchors the grace deadline for a pre-existing account via
// max(totpEnforcementAnnouncedAt, created_at) — simply setting
// created_at far in the past does NOT move the deadline earlier (the
// max() means today's totpEnforcementAnnouncedAt would still dominate),
// which is correct: nobody should be retroactively blocked just because
// their account happens to be old. To actually exercise "past grace",
// this test simulates the feature having been announced 15 days ago.
func TestRequireTOTPEnrollment_StaffPastGraceIsBlocked(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	ctx := context.Background()
	_, _, userID := createTestLoginUser(t, h)

	original := totpEnforcementAnnouncedAt
	totpEnforcementAnnouncedAt = time.Now().Add(-15 * 24 * time.Hour)
	t.Cleanup(func() { totpEnforcementAnnouncedAt = original })
	// created_at must predate the (simulated) announcement too, or
	// max(announcedAt, created_at) picks the brand-new created_at
	// instead and the deadline would still be in the future.
	if _, err := h.repo.pool.Exec(ctx, `UPDATE users SET created_at = $2 WHERE id = $1`,
		userID, totpEnforcementAnnouncedAt.Add(-5*24*time.Hour)); err != nil {
		t.Fatalf("force created_at before the simulated announcement: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := h.RequireTOTPEnrollment(next)
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), &Claims{UserID: userID, Role: RoleStudioStaff})
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (past grace period), body=%s", rec.Code, rec.Body.String())
	}
}
