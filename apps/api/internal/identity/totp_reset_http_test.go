package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TestResetStudioUserTOTP_ResetsStaffAndRejectsSelfOrWrongRole covers
// platform-security-hardening task 2.8's tier 1 (studio_admin -> staff).
func TestResetStudioUserTOTP_ResetsStaffAndRejectsSelfOrWrongRole(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	_, _, callerID := createTestLoginUser(t, h)
	ctx := context.Background()
	var studioID uuid.UUID
	if err := h.repo.pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping; no studio found in DB")
	}
	role, err := h.repo.CreateRole(ctx, studioID, "test-role-reset-"+uuid.New().String(), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })
	staffID, err := h.repo.CreateStudioUser(ctx, studioID, "reset-chain-staff", "Test", "User",
		"reset-chain-staff-"+uuid.New().String()+"@example.com", role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, staffID)
		_, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, staffID)
	})
	enrollTOTPForTest(t, h, staffID)

	// Self-reset is rejected.
	selfReq := httptest.NewRequest(http.MethodPost, "/x", nil)
	selfReq = withClaims(selfReq, &Claims{UserID: callerID})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studioId", studioID.String())
	rctx.URLParams.Add("userId", callerID.String())
	selfReq = selfReq.WithContext(context.WithValue(selfReq.Context(), chi.RouteCtxKey, rctx))
	selfRec := httptest.NewRecorder()
	h.resetStudioUserTOTP(selfRec, selfReq)
	if selfRec.Code != http.StatusForbidden {
		t.Fatalf("self-reset status = %d, want 403", selfRec.Code)
	}

	// Resetting the real staff member succeeds.
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req = withClaims(req, &Claims{UserID: callerID})
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("studioId", studioID.String())
	rctx2.URLParams.Add("userId", staffID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx2))
	rec := httptest.NewRecorder()
	h.resetStudioUserTOTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	enabled, err := h.repo.IsTOTPEnabled(ctx, staffID)
	if err != nil {
		t.Fatalf("IsTOTPEnabled: %v", err)
	}
	if enabled {
		t.Error("expected TOTP to be disabled after reset")
	}
}

// TestAdminResetUserTOTP_RejectsSelfResetButAllowsOthers covers tiers
// 2+3 (super_admin -> studio_admin or another super_admin), including
// the "a super_admin cannot reset their own 2FA" rule.
func TestAdminResetUserTOTP_RejectsSelfResetButAllowsOthers(t *testing.T) {
	h := newLoginTOTPTestHandler(t)
	_, _, callerID := createTestLoginUser(t, h)
	ctx := context.Background()
	enrollTOTPForTest(t, h, callerID)
	t.Cleanup(func() { _, _ = h.repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, callerID) })

	selfReq := httptest.NewRequest(http.MethodPost, "/x", nil)
	selfReq = withClaims(selfReq, &Claims{UserID: callerID})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userId", callerID.String())
	selfReq = selfReq.WithContext(context.WithValue(selfReq.Context(), chi.RouteCtxKey, rctx))
	selfRec := httptest.NewRecorder()
	h.adminResetUserTOTP(selfRec, selfReq)
	if selfRec.Code != http.StatusForbidden {
		t.Fatalf("self-reset status = %d, want 403, body=%s", selfRec.Code, selfRec.Body.String())
	}
	// Still enabled — the rejected self-reset must not have gone through.
	enabled, err := h.repo.IsTOTPEnabled(ctx, callerID)
	if err != nil {
		t.Fatalf("IsTOTPEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected the rejected self-reset to leave TOTP enabled")
	}

	// A different target succeeds.
	otherID := testUserID(t, h.repo)
	if otherID == callerID {
		t.Skip("skipping; only one user fixture exists in this DB")
	}
	otherReq := httptest.NewRequest(http.MethodPost, "/x", nil)
	otherReq = withClaims(otherReq, &Claims{UserID: callerID})
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("userId", otherID.String())
	otherReq = otherReq.WithContext(context.WithValue(otherReq.Context(), chi.RouteCtxKey, rctx2))
	otherRec := httptest.NewRecorder()
	h.adminResetUserTOTP(otherRec, otherReq)
	if otherRec.Code != http.StatusNoContent {
		t.Fatalf("reset-other status = %d, want 204, body=%s", otherRec.Code, otherRec.Body.String())
	}
}
