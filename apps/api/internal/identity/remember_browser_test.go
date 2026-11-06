package identity

import (
	"context"
	"testing"
)

func TestRememberBrowserToken_CreateValidateRevoke(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM remember_browser_tokens WHERE user_id = $1`, userID) })

	token, err := repo.CreateRememberBrowserToken(ctx, userID)
	if err != nil {
		t.Fatalf("CreateRememberBrowserToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	valid, err := repo.IsRememberBrowserTokenValid(ctx, userID, token)
	if err != nil {
		t.Fatalf("IsRememberBrowserTokenValid: %v", err)
	}
	if !valid {
		t.Error("expected a freshly created token to be valid")
	}

	// Wrong token, right user: invalid.
	valid, err = repo.IsRememberBrowserTokenValid(ctx, userID, "not-the-real-token")
	if err != nil {
		t.Fatalf("IsRememberBrowserTokenValid (wrong token): %v", err)
	}
	if valid {
		t.Error("expected a wrong token to be invalid")
	}

	// Empty token: invalid, no DB round-trip surprises.
	valid, err = repo.IsRememberBrowserTokenValid(ctx, userID, "")
	if err != nil {
		t.Fatalf("IsRememberBrowserTokenValid (empty): %v", err)
	}
	if valid {
		t.Error("expected an empty token to be invalid")
	}

	if err := repo.RevokeAllRememberBrowserTokens(ctx, userID); err != nil {
		t.Fatalf("RevokeAllRememberBrowserTokens: %v", err)
	}
	valid, err = repo.IsRememberBrowserTokenValid(ctx, userID, token)
	if err != nil {
		t.Fatalf("IsRememberBrowserTokenValid (after revoke): %v", err)
	}
	if valid {
		t.Error("expected the token to be invalid after RevokeAllRememberBrowserTokens")
	}
}

func TestRememberBrowserToken_IsolatedPerUser(t *testing.T) {
	repo, _, studioID := testRepo(t)
	ctx := context.Background()
	userA := testUserID(t, repo)

	role, err := repo.CreateRole(ctx, studioID, "test-role-remember-"+userA.String(), "", nil)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM studio_roles WHERE id = $1`, role.ID) })
	userB, err := repo.CreateStudioUser(ctx, studioID, "remember-browser-test-b", "Test", "User",
		"remember-browser-test-b-"+userA.String()+"@example.com", role.ID, nil)
	if err != nil {
		t.Fatalf("CreateStudioUser: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userB) })
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM remember_browser_tokens WHERE user_id = $1`, userA) })

	token, err := repo.CreateRememberBrowserToken(ctx, userA)
	if err != nil {
		t.Fatalf("CreateRememberBrowserToken: %v", err)
	}
	valid, err := repo.IsRememberBrowserTokenValid(ctx, userB, token)
	if err != nil {
		t.Fatalf("IsRememberBrowserTokenValid(userB, userA's token): %v", err)
	}
	if valid {
		t.Error("expected userA's token to be invalid for userB")
	}
}
