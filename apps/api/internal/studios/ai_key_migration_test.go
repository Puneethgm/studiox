package studios

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestMigrateLegacyAIProviderKeys_MovesLegacyValueIntoNewStore covers task
// 2.5: a provider key still sitting in the old platform_settings storage
// must end up readable via GetPlatformAIProviderKey (the new envelope-
// encrypted path) after migration, and the run must be idempotent.
func TestMigrateLegacyAIProviderKeys_MovesLegacyValueIntoNewStore(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "gemini" // pick one unlikely to collide with manual testing of "groq"
	const settingKey = "gemini_api_key"
	const legacyPlainKey = "AIzaSy_test_legacy_migration_value_123"

	// Snapshot + clear both stores for this provider so the test starts
	// from a known state, restoring on cleanup.
	var origSettingValue string
	var hadSetting bool
	if err := pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, settingKey).Scan(&origSettingValue); err == nil {
		hadSetting = true
	}
	var origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus string
	var hadKeyRow bool
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origLastFour, &origStatus); err == nil {
		hadKeyRow = true
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if hadSetting {
			_, _ = pool.Exec(ctx, `UPDATE platform_settings SET value = $2 WHERE key = $1`, settingKey, origSettingValue)
		} else {
			_, _ = pool.Exec(ctx, `DELETE FROM platform_settings WHERE key = $1`, settingKey)
		}
		if hadKeyRow {
			_, _ = pool.Exec(ctx, `
				INSERT INTO platform_ai_keys (provider, key_ciphertext, data_key_ciphertext, key_fingerprint, status)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (provider) DO UPDATE SET key_ciphertext = EXCLUDED.key_ciphertext,
					data_key_ciphertext = EXCLUDED.data_key_ciphertext, key_fingerprint = EXCLUDED.key_fingerprint, status = EXCLUDED.status
			`, provider, origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus)
		} else {
			_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditSaved)
	})

	if _, err := pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider); err != nil {
		t.Fatalf("clear platform_ai_keys row: %v", err)
	}

	// Seed the legacy row the way SetPlatformAIProviderKey used to write it
	// (settings table, single-static-key AES via repo.encryptOrPlain).
	encLegacy, err := repo.encryptOrPlain(legacyPlainKey)
	if err != nil {
		t.Fatalf("encryptOrPlain legacy key: %v", err)
	}
	var actorID *uuid.UUID
	if err := repo.UpdatePlatformSetting(ctx, settingKey, encLegacy, actorID); err != nil {
		t.Fatalf("seed legacy platform_settings row: %v", err)
	}

	migrated, err := repo.MigrateLegacyAIProviderKeys(ctx)
	if err != nil {
		t.Fatalf("MigrateLegacyAIProviderKeys: %v", err)
	}
	found := false
	for _, p := range migrated {
		if p == provider {
			found = true
		}
	}
	if !found {
		t.Fatalf("MigrateLegacyAIProviderKeys did not report %q as migrated; got %v", provider, migrated)
	}

	got, err := repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey after migration: %v", err)
	}
	if got != legacyPlainKey {
		t.Errorf("GetPlatformAIProviderKey after migration = %q, want %q", got, legacyPlainKey)
	}

	// Idempotency: running it again must not report the provider again
	// (its platform_ai_keys row already exists).
	migratedAgain, err := repo.MigrateLegacyAIProviderKeys(ctx)
	if err != nil {
		t.Fatalf("MigrateLegacyAIProviderKeys (second run): %v", err)
	}
	for _, p := range migratedAgain {
		if p == provider {
			t.Errorf("second MigrateLegacyAIProviderKeys run re-migrated %q; should have been a no-op", provider)
		}
	}
}
