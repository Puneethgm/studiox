package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestHasPriorSessionFromDevice covers platform-security-hardening task
// 1.7's new-device detection: a browser string is "unseen" until a
// session row exists recording it.
func TestHasPriorSessionFromDevice(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)
	browser := "TestBrowser/1.0 " + uuid.New().String() // unique per run, never collides with real traffic

	seen, err := repo.HasPriorSessionFromDevice(ctx, userID, browser)
	if err != nil {
		t.Fatalf("HasPriorSessionFromDevice (before any session): %v", err)
	}
	if seen {
		t.Fatal("expected a never-used browser string to be unseen")
	}

	sessionID := uuid.New()
	if err := repo.CreateSession(ctx, sessionID, userID, "", browser, "", "", false); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID) })

	seen, err = repo.HasPriorSessionFromDevice(ctx, userID, browser)
	if err != nil {
		t.Fatalf("HasPriorSessionFromDevice (after session created): %v", err)
	}
	if !seen {
		t.Error("expected the browser string to be recognized as seen once a session recorded it")
	}

	// A different browser string for the same user is still unseen.
	otherBrowser := "OtherTestBrowser/2.0 " + uuid.New().String()
	seen, err = repo.HasPriorSessionFromDevice(ctx, userID, otherBrowser)
	if err != nil {
		t.Fatalf("HasPriorSessionFromDevice (different browser): %v", err)
	}
	if seen {
		t.Error("expected a different, never-used browser string to remain unseen")
	}
}
