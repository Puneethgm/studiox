package studios

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestRotatePlatformAIProviderKey_ReplacesKeyAndAudits covers task 2.6: a
// rotated key must round-trip to its new value (the old key stops being
// used for new calls — there's only ever one row per provider, so the old
// ciphertext is simply gone) and must record a "rotated" audit action,
// distinct from the initial "saved" one.
func TestRotatePlatformAIProviderKey_ReplacesKeyAndAudits(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "openai"
	cleanupProviderKey(t, pool, provider)

	if err := repo.SetPlatformAIProviderKey(ctx, provider, "sk-original-value-aaaa", nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKey (initial save): %v", err)
	}
	if err := repo.RotatePlatformAIProviderKey(ctx, provider, "sk-rotated-value-bbbb", nil); err != nil {
		t.Fatalf("RotatePlatformAIProviderKey: %v", err)
	}

	got, err := repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey after rotation: %v", err)
	}
	if got != "sk-rotated-value-bbbb" {
		t.Errorf("after rotation, GetPlatformAIProviderKey = %q, want the new value", got)
	}

	var savedCount, rotatedCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditSaved).Scan(&savedCount); err != nil {
		t.Fatalf("query saved audit count: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditRotated).Scan(&rotatedCount); err != nil {
		t.Fatalf("query rotated audit count: %v", err)
	}
	if savedCount != 1 {
		t.Errorf("saved audit count = %d, want 1", savedCount)
	}
	if rotatedCount != 1 {
		t.Errorf("rotated audit count = %d, want 1", rotatedCount)
	}
}

// TestSetPlatformAIProviderKeyStatus_DisableMakesKeyUnavailable covers
// task 2.6's disable action: a disabled key must make
// GetPlatformAIProviderKey report "not configured" (empty string, no
// error) even though the ciphertext is still stored, and re-enabling must
// restore it without needing the key re-entered.
func TestSetPlatformAIProviderKeyStatus_DisableMakesKeyUnavailable(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "openai"
	cleanupProviderKey(t, pool, provider)

	if err := repo.SetPlatformAIProviderKey(ctx, provider, "sk-disable-test-cccc", nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKey: %v", err)
	}

	if err := repo.SetPlatformAIProviderKeyStatus(ctx, provider, AIKeyStatusDisabled, nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKeyStatus(disabled): %v", err)
	}
	got, err := repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey while disabled: %v", err)
	}
	if got != "" {
		t.Errorf("GetPlatformAIProviderKey while disabled = %q, want empty", got)
	}

	if err := repo.SetPlatformAIProviderKeyStatus(ctx, provider, AIKeyStatusActive, nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKeyStatus(active): %v", err)
	}
	got, err = repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey after re-enable: %v", err)
	}
	if got != "sk-disable-test-cccc" {
		t.Errorf("GetPlatformAIProviderKey after re-enable = %q, want the original value restored without re-entry", got)
	}

	var disabledCount, enabledCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditDisabled).Scan(&disabledCount); err != nil {
		t.Fatalf("query disabled audit count: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditEnabled).Scan(&enabledCount); err != nil {
		t.Fatalf("query enabled audit count: %v", err)
	}
	if disabledCount != 1 {
		t.Errorf("disabled audit count = %d, want 1", disabledCount)
	}
	if enabledCount != 1 {
		t.Errorf("enabled audit count = %d, want 1", enabledCount)
	}
}

// TestSetPlatformAIProviderKeyStatus_NoExistingKey confirms a status
// change on a provider with no saved key fails clearly rather than
// silently creating a phantom row.
func TestSetPlatformAIProviderKeyStatus_NoExistingKey(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	const provider = "openai"
	cleanupProviderKey(t, pool, provider)

	if err := repo.SetPlatformAIProviderKeyStatus(context.Background(), provider, AIKeyStatusDisabled, nil); err == nil {
		t.Error("expected an error disabling a provider with no saved key, got nil")
	}
}

// cleanupProviderKey deletes any row (and its audit trail) for provider up
// front, snapshotting and restoring the prior state on test cleanup — used
// by lifecycle tests that need to start from "no key saved" and must not
// permanently affect a provider another test or manual session is using.
func cleanupProviderKey(t *testing.T, pool *pgxpool.Pool, provider string) {
	t.Helper()
	ctx := context.Background()

	var origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus string
	var hadRow bool
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origLastFour, &origStatus); err == nil {
		hadRow = true
	}
	if _, err := pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider); err != nil {
		t.Fatalf("cleanupProviderKey: clear existing row: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider)
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = $1`, provider)
		if hadRow {
			_, _ = pool.Exec(ctx, `
				INSERT INTO platform_ai_keys (provider, key_ciphertext, data_key_ciphertext, key_fingerprint, status)
				VALUES ($1, $2, $3, $4, $5)
			`, provider, origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus)
		}
	})
}
