package identity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestListUserIDsByRole_And_RevokeSessionsForRole covers platform-
// security-hardening task 1.4: deactivating a role or changing its
// permissions must end every currently-assigned user's sessions, not
// just block their next login (the gap setRoleActive's own doc comment
// used to describe).
func TestListUserIDsByRole_And_RevokeSessionsForRole(t *testing.T) {
	repo, _, studioID := testRepo(t)
	sessions := testSessionStore(t, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	h := &Handler{repo: repo, sessions: sessions, tokens: tokens}
	ctx := context.Background()

	role, err := repo.CreateRole(ctx, studioID, fmt.Sprintf("test-role-%s", uuid.New()), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	email := fmt.Sprintf("revoke-trigger-test-%s@example.com", uuid.New())
	userID, err := repo.CreateStudioUser(ctx, studioID, "revoke-trigger-test", "Test", "User", email, role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	ids, err := repo.ListUserIDsByRole(ctx, role.ID)
	if err != nil {
		t.Fatalf("ListUserIDsByRole: %v", err)
	}
	if len(ids) != 1 || ids[0] != userID {
		t.Fatalf("ListUserIDsByRole = %v, want [%v]", ids, userID)
	}

	sessionID := uuid.New()
	if err := repo.CreateSession(ctx, sessionID, userID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	if err := h.revokeSessionsForRole(ctx, role.ID, "role deactivated"); err != nil {
		t.Fatalf("revokeSessionsForRole: %v", err)
	}

	revoked, err := sessions.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked: %v", err)
	}
	if !revoked {
		t.Error("expected the role-assigned user's session to be revoked")
	}
	got, err := repo.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.IsActive() {
		t.Error("expected the durable session record to be revoked too")
	}
	if got.RevokedReason != "role deactivated" {
		t.Errorf("RevokedReason = %q, want %q", got.RevokedReason, "role deactivated")
	}
}

// TestConsumePasswordResetToken_ReturnsUserID covers the signature change
// behind task 1.4's resetPassword wiring: the caller needs the affected
// user's ID to also revoke their sessions.
func TestConsumePasswordResetToken_ReturnsUserID(t *testing.T) {
	repo, _, studioID := testRepo(t)
	ctx := context.Background()

	role, err := repo.CreateRole(ctx, studioID, fmt.Sprintf("test-role-%s", uuid.New()), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })

	email := fmt.Sprintf("reset-token-test-%s@example.com", uuid.New())
	userID, err := repo.CreateStudioUser(ctx, studioID, "reset-token-test", "Test", "User", email, role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	rawToken, err := repo.CreatePasswordResetToken(ctx, userID, time.Hour)
	if err != nil {
		t.Fatalf("CreatePasswordResetToken: %v", err)
	}

	hash, err := HashPassword("a-brand-new-password-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	gotUserID, err := repo.ConsumePasswordResetToken(ctx, rawToken, hash)
	if err != nil {
		t.Fatalf("ConsumePasswordResetToken: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("ConsumePasswordResetToken returned userID %v, want %v", gotUserID, userID)
	}

	// A used/invalid token returns uuid.Nil alongside the error.
	gotUserID, err = repo.ConsumePasswordResetToken(ctx, rawToken, hash)
	if err == nil {
		t.Fatal("expected an error reusing an already-consumed reset token")
	}
	if gotUserID != uuid.Nil {
		t.Errorf("expected uuid.Nil on error, got %v", gotUserID)
	}
}
