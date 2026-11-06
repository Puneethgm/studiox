package llmgateway

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

// TestSetPlatformAIProviderKey_FingerprintNeverContainsRawKey covers
// platform-security-hardening task 0.2 from the gateway side: a provider
// key saved through studios.Repo (the same repo the gateway reads routes
// and keys from) must never be retrievable as a reversible last-4-chars
// value — only a non-reversible fingerprint, and GetPlatformAIProviderKeyFingerprint
// must never need to decrypt the key to answer.
func TestSetPlatformAIProviderKey_FingerprintNeverContainsRawKey(t *testing.T) {
	_ = godotenv.Load("../../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
	}
	dsn := "postgres://" + os.Getenv("POSTGRES_USER") + ":" + os.Getenv("POSTGRES_PASSWORD") +
		"@" + os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT") + "/" + os.Getenv("POSTGRES_DB") + "?sslmode=disable"
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	studiosRepo := studios.NewRepo(pool, cipher)

	const provider = "gemini" // real entry in platformAIProviderKeyNames, distinct from other tests' "openai"/"groq" to avoid cross-package races
	const testKey = "gem-test-fingerprint-only-zz998877"

	var hadRow bool
	var origKeyCiphertext, origDataKeyCiphertext, origFingerprint, origStatus string
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origFingerprint, &origStatus); err == nil {
		hadRow = true
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if hadRow {
			_, _ = pool.Exec(ctx, `
				UPDATE platform_ai_keys SET key_ciphertext=$2, data_key_ciphertext=$3, key_fingerprint=$4, status=$5 WHERE provider=$1
			`, provider, origKeyCiphertext, origDataKeyCiphertext, origFingerprint, origStatus)
		} else {
			_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = $1`, provider)
	})

	if err := studiosRepo.SetPlatformAIProviderKey(ctx, provider, testKey, nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKey: %v", err)
	}

	hasKey, fingerprint, err := studiosRepo.GetPlatformAIProviderKeyFingerprint(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKeyFingerprint: %v", err)
	}
	if !hasKey {
		t.Fatal("expected hasKey=true after SetPlatformAIProviderKey")
	}
	if fingerprint == "" {
		t.Fatal("expected a non-empty fingerprint")
	}
	if strings.Contains(testKey, fingerprint) || strings.Contains(fingerprint, testKey) {
		t.Errorf("fingerprint %q is derivable from/to the raw key %q", fingerprint, testKey)
	}
	if fingerprint == testKey[len(testKey)-4:] {
		t.Error("fingerprint equals the old last-4-characters value — regression to keySuffix() behavior")
	}

	// Disabling the key must make GetPlatformAIProviderKeyFingerprint report
	// hasKey=false without touching the fingerprint, matching
	// GetPlatformAIProviderKey's existing disabled-key-returns-empty
	// behavior (repo.go's GetPlatformAIProviderKey) — the two "is this key
	// usable" answers must never disagree.
	if err := studiosRepo.SetPlatformAIProviderKeyStatus(ctx, provider, studios.AIKeyStatusDisabled, nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKeyStatus: %v", err)
	}
	hasKey, _, err = studiosRepo.GetPlatformAIProviderKeyFingerprint(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKeyFingerprint after disable: %v", err)
	}
	if hasKey {
		t.Error("expected hasKey=false once the key is disabled")
	}
}
