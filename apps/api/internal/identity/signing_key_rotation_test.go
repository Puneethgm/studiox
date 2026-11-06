package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestTokenIssuer_SurvivesGracefulRotation_ButNotRotateAndPurge covers
// platform-security-hardening task 1.9 end to end: a token issued before
// a rotation keeps verifying after a graceful Rotate, but stops
// verifying after a RotateAndPurge (the platform-wide "end all
// sessions" button's extra layer).
func TestTokenIssuer_SurvivesGracefulRotation_ButNotRotateAndPurge(t *testing.T) {
	keys := testSigningKeyStore(t, "env-secret-value-at-least-32-bytes-long")
	tokens := NewTokenIssuer("unused-static-fallback-secret-32-bytes-ok", time.Hour, time.Hour, 24*time.Hour, testCipher(t))
	tokens.WithSigningKeyStore(keys)
	ctx := context.Background()

	u := &User{ID: uuid.New(), Role: RoleStudioAdmin}
	token, _, _, _, _, err := tokens.Issue(ctx, u, nil, nil)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := tokens.Parse(ctx, token); err != nil {
		t.Fatalf("expected the freshly issued token to parse, got: %v", err)
	}

	if _, err := keys.Rotate(ctx); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if _, err := tokens.Parse(ctx, token); err != nil {
		t.Errorf("expected the pre-rotation token to still parse after a graceful Rotate, got: %v", err)
	}

	// A new token issued after the rotation should be signed with the
	// NEW key, not the old envKeyID one.
	newToken, _, _, _, _, err := tokens.Issue(ctx, u, nil, nil)
	if err != nil {
		t.Fatalf("issue after rotate: %v", err)
	}
	if _, err := tokens.Parse(ctx, newToken); err != nil {
		t.Errorf("expected the post-rotation token to parse, got: %v", err)
	}

	if _, err := keys.RotateAndPurge(ctx); err != nil {
		t.Fatalf("RotateAndPurge: %v", err)
	}
	if _, err := tokens.Parse(ctx, token); err == nil {
		t.Error("expected the original (pre-rotation) token to fail verification after RotateAndPurge")
	}
	if _, err := tokens.Parse(ctx, newToken); err == nil {
		t.Error("expected the post-rotation (but pre-purge) token to also fail verification after RotateAndPurge")
	}
}
