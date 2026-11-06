package identity

import (
	"context"
	"errors"
	"testing"
)

func TestTOTP_EnrollmentLifecycle(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID) })

	if _, err := repo.GetTOTP(ctx, userID); !errors.Is(err, ErrTOTPNotEnrolled) {
		t.Fatalf("GetTOTP before any enrollment = %v, want ErrTOTPNotEnrolled", err)
	}
	if enabled, err := repo.IsTOTPEnabled(ctx, userID); err != nil || enabled {
		t.Fatalf("IsTOTPEnabled before enrollment = (%v, %v), want (false, nil)", enabled, err)
	}

	if err := repo.SaveTOTPSecretPending(ctx, userID, "ciphertext-1"); err != nil {
		t.Fatalf("SaveTOTPSecretPending: %v", err)
	}
	rec, err := repo.GetTOTP(ctx, userID)
	if err != nil {
		t.Fatalf("GetTOTP after pending save: %v", err)
	}
	if rec.Enabled {
		t.Error("expected enabled=false immediately after SaveTOTPSecretPending")
	}
	if rec.SecretCiphertext != "ciphertext-1" {
		t.Errorf("SecretCiphertext = %q, want %q", rec.SecretCiphertext, "ciphertext-1")
	}

	// Re-saving before confirmation (e.g. the user re-scans the QR code)
	// replaces the pending secret.
	if err := repo.SaveTOTPSecretPending(ctx, userID, "ciphertext-2"); err != nil {
		t.Fatalf("SaveTOTPSecretPending (replace): %v", err)
	}
	rec, _ = repo.GetTOTP(ctx, userID)
	if rec.SecretCiphertext != "ciphertext-2" {
		t.Errorf("SecretCiphertext after replace = %q, want %q", rec.SecretCiphertext, "ciphertext-2")
	}

	if err := repo.EnableTOTP(ctx, userID); err != nil {
		t.Fatalf("EnableTOTP: %v", err)
	}
	if enabled, err := repo.IsTOTPEnabled(ctx, userID); err != nil || !enabled {
		t.Fatalf("IsTOTPEnabled after EnableTOTP = (%v, %v), want (true, nil)", enabled, err)
	}

	if err := repo.MarkTOTPStepUsed(ctx, userID, 12345); err != nil {
		t.Fatalf("MarkTOTPStepUsed: %v", err)
	}
	rec, _ = repo.GetTOTP(ctx, userID)
	if rec.LastUsedStep == nil || *rec.LastUsedStep != 12345 {
		t.Errorf("LastUsedStep = %v, want 12345", rec.LastUsedStep)
	}

	if err := repo.DisableTOTP(ctx, userID); err != nil {
		t.Fatalf("DisableTOTP: %v", err)
	}
	if _, err := repo.GetTOTP(ctx, userID); !errors.Is(err, ErrTOTPNotEnrolled) {
		t.Errorf("GetTOTP after DisableTOTP = %v, want ErrTOTPNotEnrolled", err)
	}
}

func TestEnableTOTP_WithoutPendingSecretReturnsNotEnrolled(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)
	if err := repo.EnableTOTP(ctx, userID); !errors.Is(err, ErrTOTPNotEnrolled) {
		t.Errorf("EnableTOTP with no pending row = %v, want ErrTOTPNotEnrolled", err)
	}
}

func TestRecoveryCodes_CreateConsumeSingleUseAndCount(t *testing.T) {
	repo, _, _ := testRepo(t)
	ctx := context.Background()
	userID := testUserID(t, repo)
	t.Cleanup(func() { _, _ = repo.pool.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID) })

	codes, err := generateRecoveryCodes()
	if err != nil {
		t.Fatalf("generateRecoveryCodes: %v", err)
	}
	hashes := make([]string, len(codes))
	for i, c := range codes {
		h, err := HashPassword(normalizeRecoveryCode(c))
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		hashes[i] = h
	}
	if err := repo.CreateRecoveryCodes(ctx, userID, hashes); err != nil {
		t.Fatalf("CreateRecoveryCodes: %v", err)
	}

	count, err := repo.CountUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		t.Fatalf("CountUnusedRecoveryCodes: %v", err)
	}
	if count != len(codes) {
		t.Fatalf("CountUnusedRecoveryCodes = %d, want %d", count, len(codes))
	}

	// Consume the first code — accepted exactly once.
	ok, err := repo.ConsumeRecoveryCode(ctx, userID, normalizeRecoveryCode(codes[0]))
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode: %v", err)
	}
	if !ok {
		t.Fatal("expected the first recovery code to be accepted")
	}
	count, _ = repo.CountUnusedRecoveryCodes(ctx, userID)
	if count != len(codes)-1 {
		t.Errorf("CountUnusedRecoveryCodes after one consumed = %d, want %d", count, len(codes)-1)
	}

	// Reusing the same code fails.
	ok, err = repo.ConsumeRecoveryCode(ctx, userID, normalizeRecoveryCode(codes[0]))
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode (reuse): %v", err)
	}
	if ok {
		t.Error("expected reusing an already-consumed recovery code to fail")
	}

	// An unrelated, never-issued code fails.
	ok, err = repo.ConsumeRecoveryCode(ctx, userID, "ZZZZZZZZZZ")
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode (bogus): %v", err)
	}
	if ok {
		t.Error("expected a bogus code to be rejected")
	}

	// Replacing the codes (CreateRecoveryCodes again) clears the old set.
	newCodes, _ := generateRecoveryCodes()
	newHashes := make([]string, len(newCodes))
	for i, c := range newCodes {
		h, _ := HashPassword(normalizeRecoveryCode(c))
		newHashes[i] = h
	}
	if err := repo.CreateRecoveryCodes(ctx, userID, newHashes); err != nil {
		t.Fatalf("CreateRecoveryCodes (replace): %v", err)
	}
	count, _ = repo.CountUnusedRecoveryCodes(ctx, userID)
	if count != len(newCodes) {
		t.Errorf("CountUnusedRecoveryCodes after replace = %d, want %d", count, len(newCodes))
	}
	// An old code (from before the replace) no longer works.
	ok, _ = repo.ConsumeRecoveryCode(ctx, userID, normalizeRecoveryCode(codes[1]))
	if ok {
		t.Error("expected an old (pre-replace) code to no longer be valid")
	}
}
