package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func testTwoFALockoutStore(t *testing.T) *TwoFALockoutStore {
	t.Helper()
	return NewTwoFALockoutStore(testRawRedisClient(t))
}

func TestTwoFALockoutStore_LocksAfterFiveFailures(t *testing.T) {
	s := testTwoFALockoutStore(t)
	ctx := context.Background()
	userID := uuid.New()
	t.Cleanup(func() { _ = s.Reset(ctx, userID) })

	locked, err := s.IsLocked(ctx, userID)
	if err != nil {
		t.Fatalf("IsLocked (before any failure): %v", err)
	}
	if locked {
		t.Fatal("expected a never-failed account to not be locked")
	}

	var justLocked bool
	for i := 0; i < totpLockoutThreshold; i++ {
		justLocked, err = s.RecordFailure(ctx, userID)
		if err != nil {
			t.Fatalf("RecordFailure (attempt %d): %v", i+1, err)
		}
		if i < totpLockoutThreshold-1 && justLocked {
			t.Errorf("RecordFailure reported justLocked=true too early, on attempt %d", i+1)
		}
	}
	if !justLocked {
		t.Error("expected the 5th failure to report justLocked=true")
	}

	locked, err = s.IsLocked(ctx, userID)
	if err != nil {
		t.Fatalf("IsLocked (after 5 failures): %v", err)
	}
	if !locked {
		t.Error("expected the account to be locked after 5 failures")
	}

	// A 6th failure must not report justLocked again (one-time
	// notification, not one per attempt while still locked).
	justLocked, err = s.RecordFailure(ctx, userID)
	if err != nil {
		t.Fatalf("RecordFailure (6th): %v", err)
	}
	if justLocked {
		t.Error("expected justLocked=false on the 6th failure — the notification already fired on the 5th")
	}
}

func TestTwoFALockoutStore_ResetClearsLockout(t *testing.T) {
	s := testTwoFALockoutStore(t)
	ctx := context.Background()
	userID := uuid.New()
	t.Cleanup(func() { _ = s.Reset(ctx, userID) })

	for i := 0; i < totpLockoutThreshold; i++ {
		if _, err := s.RecordFailure(ctx, userID); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}
	locked, _ := s.IsLocked(ctx, userID)
	if !locked {
		t.Fatal("expected the account to be locked before Reset")
	}

	if err := s.Reset(ctx, userID); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	locked, err := s.IsLocked(ctx, userID)
	if err != nil {
		t.Fatalf("IsLocked (after Reset): %v", err)
	}
	if locked {
		t.Error("expected the account to be unlocked after Reset")
	}
}

func TestTwoFALockoutStore_IndependentPerAccount(t *testing.T) {
	s := testTwoFALockoutStore(t)
	ctx := context.Background()
	userA, userB := uuid.New(), uuid.New()
	t.Cleanup(func() {
		_ = s.Reset(ctx, userA)
		_ = s.Reset(ctx, userB)
	})

	for i := 0; i < totpLockoutThreshold; i++ {
		if _, err := s.RecordFailure(ctx, userA); err != nil {
			t.Fatalf("RecordFailure(userA): %v", err)
		}
	}
	lockedA, _ := s.IsLocked(ctx, userA)
	lockedB, _ := s.IsLocked(ctx, userB)
	if !lockedA {
		t.Error("expected userA to be locked")
	}
	if lockedB {
		t.Error("expected userB to be unaffected by userA's failures")
	}
}
