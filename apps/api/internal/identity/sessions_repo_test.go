package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// testUserID returns a real existing user's ID — sessions.user_id has an
// FK to users(id), same reasoning as testRepo grabbing a real studio ID.
func testUserID(t *testing.T, repo *Repo) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := repo.pool.QueryRow(context.Background(), `SELECT id FROM users LIMIT 1`).Scan(&id); err != nil {
		t.Skip("skipping; no user found in DB")
	}
	return id
}

// TestSessions_CreateTouchListRevoke covers platform-security-hardening
// task 1.1/1.11: the durable Postgres session record, independent of the
// Redis-backed jti/revocation checks covered in session_test.go.
func TestSessions_CreateTouchListRevoke(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)

	id := uuid.New()
	if err := repo.CreateSession(ctx, id, userID, "iPhone", "Mobile Safari", "203.0.113.5", "SG", true); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id) })

	got, err := repo.GetSession(ctx, id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != userID || got.Device != "iPhone" || got.Browser != "Mobile Safari" || got.IP != "203.0.113.5" || got.Country != "SG" || !got.TwoFAPassed {
		t.Errorf("GetSession returned unexpected fields: %+v", got)
	}
	if !got.IsActive() {
		t.Error("a freshly created session must be active")
	}

	firstSeen := got.LastSeenAt
	if err := repo.TouchSession(ctx, id); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}
	got, err = repo.GetSession(ctx, id)
	if err != nil {
		t.Fatalf("GetSession after touch: %v", err)
	}
	if !got.LastSeenAt.After(firstSeen) && !got.LastSeenAt.Equal(firstSeen) {
		t.Errorf("TouchSession should not move last_seen_at backwards: before=%v after=%v", firstSeen, got.LastSeenAt)
	}

	list, err := repo.ListActiveSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListActiveSessions: %v", err)
	}
	found := false
	for _, s := range list {
		if s.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("ListActiveSessions did not include the session just created")
	}

	if err := repo.RevokeSession(ctx, id, "test revoke"); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	got, err = repo.GetSession(ctx, id)
	if err != nil {
		t.Fatalf("GetSession after revoke: %v", err)
	}
	if got.IsActive() {
		t.Error("session should be inactive after RevokeSession")
	}
	if got.RevokedReason != "test revoke" {
		t.Errorf("RevokedReason = %q, want %q", got.RevokedReason, "test revoke")
	}

	list, err = repo.ListActiveSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListActiveSessions after revoke: %v", err)
	}
	for _, s := range list {
		if s.ID == id {
			t.Error("ListActiveSessions must not include a revoked session")
		}
	}

	// Revoking an already-revoked session is a no-op, not an error.
	if err := repo.RevokeSession(ctx, id, "second revoke"); err != nil {
		t.Fatalf("RevokeSession (idempotent second call): %v", err)
	}
	got, _ = repo.GetSession(ctx, id)
	if got.RevokedReason != "test revoke" {
		t.Error("re-revoking an already-revoked session must not overwrite the original reason")
	}
}

// TestSessions_RevokeSessionsForUser covers the multi-session fan-out
// RevokeSessionsForUser (task 1.3) relies on: every active session for a
// user is revoked, and only that user's sessions.
func TestSessions_RevokeSessionsForUser(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)

	id1, id2 := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{id1, id2} {
		if err := repo.CreateSession(ctx, id, userID, "", "", "", "", false); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, []uuid.UUID{id1, id2})
	})

	revokedIDs, err := repo.RevokeSessionsForUser(ctx, userID, "logout-all")
	if err != nil {
		t.Fatalf("RevokeSessionsForUser: %v", err)
	}
	gotIDs := map[uuid.UUID]bool{}
	for _, id := range revokedIDs {
		gotIDs[id] = true
	}
	if !gotIDs[id1] || !gotIDs[id2] {
		t.Errorf("RevokeSessionsForUser returned %v, want it to include both %v and %v", revokedIDs, id1, id2)
	}

	for _, id := range []uuid.UUID{id1, id2} {
		got, err := repo.GetSession(ctx, id)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.IsActive() {
			t.Errorf("session %v should be revoked after RevokeSessionsForUser", id)
		}
		if got.RevokedReason != "logout-all" {
			t.Errorf("RevokedReason = %q, want %q", got.RevokedReason, "logout-all")
		}
	}

	// Calling again (no active sessions left) returns no IDs, not an error.
	revokedIDs, err = repo.RevokeSessionsForUser(ctx, userID, "logout-all again")
	if err != nil {
		t.Fatalf("RevokeSessionsForUser (no-op call): %v", err)
	}
	if len(revokedIDs) != 0 {
		t.Errorf("expected no sessions revoked on the second call, got %v", revokedIDs)
	}
}
