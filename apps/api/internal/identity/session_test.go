package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/rediscache"
)

// testRawRedisClient connects to the real dev Redis — same skip-if-
// unconfigured pattern as internal/platform/rediscache's own tests.
// Shared by every test in this package that needs Redis directly
// (testSessionStore below, and signing_keys_test.go's SigningKeyStore
// tests).
func testRawRedisClient(t *testing.T) *rediscache.Client {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		t.Skip("Skipping integration test; REDIS_PASSWORD not set")
	}
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	client := rediscache.New(fmt.Sprintf("%s:%s", host, port), password, 0)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func testSessionStore(t *testing.T, idleTimeout time.Duration) *SessionStore {
	t.Helper()
	return NewSessionStore(testRawRedisClient(t), idleTimeout)
}

func TestSessionStore_CreateExistsRevoke(t *testing.T) {
	s := testSessionStore(t, time.Minute)
	ctx := context.Background()
	jti := uuid.NewString()

	if err := s.Exists(ctx, jti); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Exists on never-created jti = %v, want ErrSessionNotFound", err)
	}

	if err := s.Create(ctx, jti, uuid.New()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Exists(ctx, jti); err != nil {
		t.Fatalf("Exists on active session: %v", err)
	}

	if err := s.Revoke(ctx, jti); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := s.Exists(ctx, jti); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Exists after Revoke = %v, want ErrSessionNotFound", err)
	}

	// Revoking an already-absent session is a no-op, not an error.
	if err := s.Revoke(ctx, jti); err != nil {
		t.Fatalf("Revoke on absent session: %v", err)
	}
}

// TestSessionStore_MarkRevokedIsRevoked covers platform-security-hardening
// task 1.3/1.11: the fast, every-request revocation check, independent of
// the jti-keyed liveness check above.
func TestSessionStore_MarkRevokedIsRevoked(t *testing.T) {
	s := testSessionStore(t, time.Minute)
	ctx := context.Background()
	sessionID := uuid.New()

	revoked, err := s.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked on never-marked session: %v", err)
	}
	if revoked {
		t.Fatal("a never-marked session must not report as revoked")
	}

	if err := s.MarkRevoked(ctx, sessionID, time.Minute); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}
	revoked, err = s.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked after MarkRevoked: %v", err)
	}
	if !revoked {
		t.Fatal("expected IsRevoked to report true after MarkRevoked")
	}

	// A different, never-marked session must be unaffected.
	other := uuid.New()
	revoked, err = s.IsRevoked(ctx, other)
	if err != nil {
		t.Fatalf("IsRevoked on unrelated session: %v", err)
	}
	if revoked {
		t.Fatal("MarkRevoked must not affect an unrelated session ID")
	}
}

func TestSessionStore_MarkRevokedExpiresAfterTTL(t *testing.T) {
	s := testSessionStore(t, time.Minute)
	ctx := context.Background()
	sessionID := uuid.New()

	if err := s.MarkRevoked(ctx, sessionID, 1500*time.Millisecond); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}
	time.Sleep(2 * time.Second)
	revoked, err := s.IsRevoked(ctx, sessionID)
	if err != nil {
		t.Fatalf("IsRevoked after TTL elapsed: %v", err)
	}
	if revoked {
		t.Error("expected the revocation record to have expired on its own TTL")
	}
}

// TestSessionStore_EntryExpiresAfterTTL confirms the Redis entry still
// self-cleans on its own TTL — it's no longer the idle-timeout enforcement
// mechanism (that's Claims.IdleExpiresAt / TokenIssuer.Refresh now), just
// housekeeping so an abandoned session's Redis record doesn't linger
// forever.
func TestSessionStore_EntryExpiresAfterTTL(t *testing.T) {
	s := testSessionStore(t, 1500*time.Millisecond)
	ctx := context.Background()
	jti := uuid.NewString()

	if err := s.Create(ctx, jti, uuid.New()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	time.Sleep(2 * time.Second)
	if err := s.Exists(ctx, jti); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Exists after TTL elapsed = %v, want ErrSessionNotFound", err)
	}
}
