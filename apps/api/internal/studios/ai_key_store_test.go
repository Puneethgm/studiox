package studios

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

func setupAIKeyStoreTest(t *testing.T) (*Repo, *pgxpool.Pool) {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
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
	return NewRepo(pool, cipher), pool
}

// TestSetGetPlatformAIProviderKey_RoundTripsAndIsNotStoredPlaintext covers
// task 2.3/2.4: saving a key through the superadmin path and reading it
// back must round-trip, must never be stored in a form containing the
// plaintext key, and must record the correct non-reversible fingerprint
// for display (platform-security-hardening task 0.2 replaced the prior
// last-4-characters reveal with this fingerprint).
func TestSetGetPlatformAIProviderKey_RoundTripsAndIsNotStoredPlaintext(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "openai" // a real entry in platformAIProviderKeyNames, chosen to avoid touching a provider that may hold a real configured key
	const testKey = "sk-test-envelope-roundtrip-abcd1234wxyz"

	var origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus string
	var hadRow bool
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origLastFour, &origStatus); err == nil {
		hadRow = true
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if hadRow {
			_, _ = pool.Exec(ctx, `UPDATE platform_ai_keys SET key_ciphertext = $2, data_key_ciphertext = $3, key_fingerprint = $4, status = $5 WHERE provider = $1`,
				provider, origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus)
		} else {
			_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = $1 AND action = $2`, provider, AIKeyAuditSaved)
	})

	if err := repo.SetPlatformAIProviderKey(ctx, provider, testKey, nil); err != nil {
		t.Fatalf("SetPlatformAIProviderKey: %v", err)
	}

	var keyCiphertext, dataKeyCiphertext, lastFour, status string
	if err := pool.QueryRow(ctx, `
		SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1
	`, provider).Scan(&keyCiphertext, &dataKeyCiphertext, &lastFour, &status); err != nil {
		t.Fatalf("read stored row: %v", err)
	}

	if strings.Contains(keyCiphertext, testKey) {
		t.Error("key_ciphertext contains the plaintext key")
	}
	if keyCiphertext == testKey {
		t.Error("key_ciphertext equals the plaintext key")
	}
	if dataKeyCiphertext == "" {
		t.Error("data_key_ciphertext is empty — envelope encryption didn't store a wrapped data key")
	}
	wantFingerprint := fingerprintSecret(testKey)
	if lastFour != wantFingerprint {
		t.Errorf("key_fingerprint = %q, want %q", lastFour, wantFingerprint)
	}
	if strings.Contains(lastFour, testKey) || strings.HasSuffix(testKey, lastFour) {
		t.Error("key_fingerprint leaks a substring of the plaintext key")
	}
	if status != AIKeyStatusActive {
		t.Errorf("status = %q, want %q", status, AIKeyStatusActive)
	}

	got, err := repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey: %v", err)
	}
	if got != testKey {
		t.Errorf("GetPlatformAIProviderKey round trip = %q, want %q", got, testKey)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM platform_ai_key_audit WHERE provider = $1 AND action = $2
	`, provider, AIKeyAuditSaved).Scan(&auditCount); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditCount == 0 {
		t.Error("no audit row recorded for the key save")
	}
}

// TestGetPlatformAIProviderKey_UnknownProvider confirms the provider
// allowlist still rejects anything outside platformAIProviderKeyNames, the
// same contract as before the table existed.
func TestGetPlatformAIProviderKey_UnknownProvider(t *testing.T) {
	repo, _ := setupAIKeyStoreTest(t)
	if _, err := repo.GetPlatformAIProviderKey(context.Background(), "not-a-real-provider"); err == nil {
		t.Error("expected an error for an unsupported provider, got nil")
	}
}

// TestGetPlatformAIProviderKey_NoRowReturnsEmptyNotError matches the
// pre-existing "provider not configured" contract: a provider with no
// saved key is not an error.
func TestGetPlatformAIProviderKey_NoRowReturnsEmptyNotError(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "claude"
	var origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus string
	var hadRow bool
	if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
		Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origLastFour, &origStatus); err == nil {
		hadRow = true
	}
	if _, err := pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider); err != nil {
		t.Fatalf("clear existing row: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		if hadRow {
			_, _ = pool.Exec(ctx, `
				INSERT INTO platform_ai_keys (provider, key_ciphertext, data_key_ciphertext, key_fingerprint, status)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (provider) DO UPDATE SET key_ciphertext = EXCLUDED.key_ciphertext,
					data_key_ciphertext = EXCLUDED.data_key_ciphertext, key_fingerprint = EXCLUDED.key_fingerprint, status = EXCLUDED.status
			`, provider, origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus)
		}
	})

	got, err := repo.GetPlatformAIProviderKey(ctx, provider)
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKey: %v", err)
	}
	if got != "" {
		t.Errorf("GetPlatformAIProviderKey with no saved key = %q, want empty", got)
	}
}
