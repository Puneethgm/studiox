package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestHandler_RevokeSessionsForUser covers platform-security-hardening
// task 1.3: a single call must revoke both the durable Postgres record
// and the fast Redis check, so IsRevoked reflects it immediately.
func TestHandler_RevokeSessionsForUser(t *testing.T) {
	repo, _, _ := testRepo(t)
	sessions := testSessionStore(t, time.Minute)
	tokens := NewTokenIssuer("test-secret-at-least-32-characters-long", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	h := &Handler{repo: repo, sessions: sessions, tokens: tokens}

	ctx := context.Background()
	userID := testUserID(t, repo)
	sessionID := uuid.New()
	if err := repo.CreateSession(ctx, sessionID, userID, "", "", "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	revoked, err := sessions.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked before revoke: %v", err)
	}
	if revoked {
		t.Fatal("session must not report revoked before RevokeSessionsForUser runs")
	}

	if err := h.RevokeSessionsForUser(ctx, userID, "password changed"); err != nil {
		t.Fatalf("RevokeSessionsForUser: %v", err)
	}

	revoked, err = sessions.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked after revoke: %v", err)
	}
	if !revoked {
		t.Fatal("expected IsRevoked to report true immediately after RevokeSessionsForUser")
	}

	got, err := repo.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.IsActive() {
		t.Error("the durable session record must also be marked revoked")
	}
	if got.RevokedReason != "password changed" {
		t.Errorf("RevokedReason = %q, want %q", got.RevokedReason, "password changed")
	}
}
