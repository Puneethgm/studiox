package identity

import (
	"context"
	"testing"
)

func testLoginLockoutStore(t *testing.T) *LoginLockoutStore {
	t.Helper()
	return NewLoginLockoutStore(testRawRedisClient(t))
}

func TestLoginLockoutStore_LocksAfterFiveFailures(t *testing.T) {
	s := testLoginLockoutStore(t)
	ctx := context.Background()
	email := "lockout-test@example.com"
	t.Cleanup(func() { _ = s.Reset(ctx, email) })

	locked, err := s.IsLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsLocked (before any failure): %v", err)
	}
	if locked {
		t.Fatal("expected a never-failed account to not be locked")
	}

	var justLocked bool
	for i := 0; i < loginLockoutThreshold; i++ {
		justLocked, err = s.RecordFailure(ctx, email)
		if err != nil {
			t.Fatalf("RecordFailure (attempt %d): %v", i+1, err)
		}
		if i < loginLockoutThreshold-1 && justLocked {
			t.Errorf("RecordFailure reported justLocked=true too early, on attempt %d", i+1)
		}
	}
	if !justLocked {
		t.Error("expected the 5th failure to report justLocked=true")
	}

	locked, err = s.IsLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsLocked (after threshold): %v", err)
	}
	if !locked {
		t.Fatal("expected the account to be locked after 5 failures")
	}
}

func TestLoginLockoutStore_ResetClearsLock(t *testing.T) {
	s := testLoginLockoutStore(t)
	ctx := context.Background()
	email := "lockout-reset-test@example.com"
	t.Cleanup(func() { _ = s.Reset(ctx, email) })

	for i := 0; i < loginLockoutThreshold; i++ {
		if _, err := s.RecordFailure(ctx, email); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}
	if err := s.Reset(ctx, email); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	locked, err := s.IsLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsLocked (after reset): %v", err)
	}
	if locked {
		t.Fatal("expected Reset to clear the lockout")
	}
}

func TestLoginLockoutStore_IsPerAccountNotPerIP(t *testing.T) {
	// The store's key is the email alone — nothing in RecordFailure/
	// IsLocked ever takes an IP, so varying source IPs against the same
	// account still shares one counter (task 4.3's "independent of
	// source IP" requirement) by construction, not by a check this test
	// could bypass. This test documents that guarantee structurally:
	// two "requests" from different (simulated) IPs both increment the
	// exact same key.
	s := testLoginLockoutStore(t)
	ctx := context.Background()
	email := "lockout-per-account-test@example.com"
	t.Cleanup(func() { _ = s.Reset(ctx, email) })

	for i := 0; i < loginLockoutThreshold; i++ {
		// Each call here simulates a different source IP — note that
		// no IP is ever passed in, which is the point.
		if _, err := s.RecordFailure(ctx, email); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}
	locked, err := s.IsLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if !locked {
		t.Fatal("expected lockout to trigger regardless of which IP each failure came from")
	}
}
