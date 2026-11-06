package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func testPendingAuthStore(t *testing.T) *PendingAuthStore {
	t.Helper()
	return NewPendingAuthStore(testRawRedisClient(t))
}

func TestPendingAuthStore_CreateResolveConsume(t *testing.T) {
	s := testPendingAuthStore(t)
	ctx := context.Background()
	userID := uuid.New()

	token, err := s.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	// Resolve does not consume — calling it twice both succeed.
	got, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != userID {
		t.Errorf("Resolve returned %v, want %v", got, userID)
	}
	got, err = s.Resolve(ctx, token)
	if err != nil {
		t.Fatalf("Resolve (second call): %v", err)
	}
	if got != userID {
		t.Errorf("Resolve (second call) returned %v, want %v", got, userID)
	}

	// Consume resolves once more, then invalidates the token.
	got, err = s.Consume(ctx, token)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if got != userID {
		t.Errorf("Consume returned %v, want %v", got, userID)
	}
	if _, err := s.Resolve(ctx, token); !errors.Is(err, ErrPendingAuthNotFound) {
		t.Errorf("Resolve after Consume = %v, want ErrPendingAuthNotFound", err)
	}
}

func TestPendingAuthStore_UnknownTokenNotFound(t *testing.T) {
	s := testPendingAuthStore(t)
	if _, err := s.Resolve(context.Background(), "this-token-was-never-issued"); !errors.Is(err, ErrPendingAuthNotFound) {
		t.Errorf("Resolve on unknown token = %v, want ErrPendingAuthNotFound", err)
	}
}
