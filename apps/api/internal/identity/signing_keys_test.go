package identity

import (
	"context"
	"testing"
	"time"
)

// testSigningKeyStore connects to the real dev Redis, same pattern as
// testSessionStore, and cleans up whatever it wrote under the signing-
// keys key so it never leaks into another test run.
func testSigningKeyStore(t *testing.T, envSecret string) *SigningKeyStore {
	t.Helper()
	redis := testRawRedisClient(t)
	s := NewSigningKeyStore(redis, envSecret, 0) // cacheTTL=0: every call hits Redis, so tests see rotations immediately
	t.Cleanup(func() { _ = redis.Del(context.Background(), signingKeysRedisKey) })
	return s
}

func TestSigningKeyStore_BootstrapsFromEnvSecretWhenNeverRotated(t *testing.T) {
	s := testSigningKeyStore(t, "env-secret-value")
	ctx := context.Background()

	kid, secret, err := s.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if kid != envKeyID {
		t.Errorf("kid = %q, want %q", kid, envKeyID)
	}
	if string(secret) != "env-secret-value" {
		t.Errorf("secret = %q, want the env secret", secret)
	}

	gotSecret, ok, err := s.Lookup(ctx, envKeyID)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ok || string(gotSecret) != "env-secret-value" {
		t.Errorf("Lookup(envKeyID) = (%q, %v), want (%q, true)", gotSecret, ok, "env-secret-value")
	}
}

func TestSigningKeyStore_RotateKeepsOldKeyVerifiable(t *testing.T) {
	s := testSigningKeyStore(t, "env-secret-value")
	ctx := context.Background()

	newKid, err := s.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newKid == envKeyID {
		t.Fatal("Rotate should mint a new kid distinct from envKeyID")
	}

	curKid, curSecret, err := s.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if curKid != newKid {
		t.Errorf("Current kid = %q, want %q", curKid, newKid)
	}
	if len(curSecret) != 32 {
		t.Errorf("rotated secret length = %d, want 32 random bytes", len(curSecret))
	}

	// The old (env) key must still verify — a graceful rotation.
	envSecret, ok, err := s.Lookup(ctx, envKeyID)
	if err != nil {
		t.Fatalf("Lookup(envKeyID) after rotate: %v", err)
	}
	if !ok || string(envSecret) != "env-secret-value" {
		t.Error("expected the pre-rotation key to remain verifiable after a graceful Rotate")
	}
}

func TestSigningKeyStore_RotateAndPurgeInvalidatesEveryOtherKey(t *testing.T) {
	s := testSigningKeyStore(t, "env-secret-value")
	ctx := context.Background()

	if _, err := s.Rotate(ctx); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	newKid, err := s.RotateAndPurge(ctx)
	if err != nil {
		t.Fatalf("RotateAndPurge: %v", err)
	}

	curKid, _, err := s.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if curKid != newKid {
		t.Errorf("Current kid = %q, want %q", curKid, newKid)
	}

	_, ok, err := s.Lookup(ctx, envKeyID)
	if err != nil {
		t.Fatalf("Lookup(envKeyID): %v", err)
	}
	if ok {
		t.Error("expected envKeyID to be purged and no longer verifiable after RotateAndPurge")
	}
}

func TestSigningKeyStore_PersistsAcrossInstances(t *testing.T) {
	redis := testRawRedisClient(t)
	ctx := context.Background()
	t.Cleanup(func() { _ = redis.Del(ctx, signingKeysRedisKey) })

	a := NewSigningKeyStore(redis, "env-secret-value", 0)
	newKid, err := a.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// A second, independent instance (simulating a different process /
	// a restart) must see the rotation via Redis, not an in-memory cache.
	b := NewSigningKeyStore(redis, "env-secret-value", 0)
	curKid, _, err := b.Current(ctx)
	if err != nil {
		t.Fatalf("Current (second instance): %v", err)
	}
	if curKid != newKid {
		t.Errorf("second instance's Current kid = %q, want %q (the rotation from the first instance)", curKid, newKid)
	}
}

func TestSigningKeyStore_CacheTTLBoundsStaleness(t *testing.T) {
	redis := testRawRedisClient(t)
	ctx := context.Background()
	t.Cleanup(func() { _ = redis.Del(ctx, signingKeysRedisKey) })

	cached := NewSigningKeyStore(redis, "env-secret-value", time.Hour) // long TTL: should NOT see a rotation immediately
	if _, _, err := cached.Current(ctx); err != nil {
		t.Fatalf("Current (prime cache): %v", err)
	}

	fresh := NewSigningKeyStore(redis, "env-secret-value", 0)
	newKid, err := fresh.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	curKid, _, err := cached.Current(ctx)
	if err != nil {
		t.Fatalf("Current (cached instance): %v", err)
	}
	if curKid == newKid {
		t.Error("expected the long-TTL cached instance to still see the pre-rotation kid immediately after another instance rotated")
	}
}
