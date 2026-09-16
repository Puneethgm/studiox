package llm

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/integrations/groq"
	"github.com/projectx/api/internal/platform/secrets"
)

// fakePlatformSettings stands in for *studios.Repo's GetPlatformSetting —
// importing internal/studios directly here would create an import cycle,
// since it depends on internal/integrations/crm, which depends on this
// package. None of the subtests below exercise Gemini, so an empty lookup
// is fine.
type fakePlatformSettings struct{}

func (fakePlatformSettings) GetPlatformSetting(ctx context.Context, key string) (string, error) {
	return "", nil
}

// TestResolveProvider_Integration confirms ai_task_configs correctly picks
// the adapter type per provider, and that an api_key_enc override takes
// priority over the platform-level key. Same DB-test shape as
// internal/integrations/crm/executor_test.go — real local Postgres,
// t.Skip if unconfigured.
func TestResolveProvider_Integration(t *testing.T) {
	_ = godotenv.Load("../../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)

	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Skip("Skipping test; no user found in DB")
	}

	// No platform Claude/Groq client configured in this test process — every
	// case below either supplies an override key or expects a clear error,
	// so a nil platform client is fine and exercises the "not configured"
	// path honestly.
	resolver := NewResolver(repo, nil, "https://api.anthropic.com/v1/messages", nil, fakePlatformSettings{})

	t.Run("no config row -> clear error, not a default guess", func(t *testing.T) {
		purpose := "test_purpose_unset_" + uuid.NewString()
		_, err := resolver.ResolveProvider(ctx, purpose)
		if err == nil {
			t.Fatal("expected an error for an unconfigured purpose, got nil")
		}
	})

	t.Run("claude with override key returns a working claudeAdapter", func(t *testing.T) {
		purpose := "test_purpose_claude_" + uuid.NewString()
		if err := repo.UpsertTaskConfig(ctx, purpose, ProviderClaude, "claude-haiku-4-5-20251001", "override-claude-key", userID); err != nil {
			t.Fatalf("upsert task config: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM ai_task_configs WHERE purpose = $1`, purpose) })

		p, err := resolver.ResolveProvider(ctx, purpose)
		if err != nil {
			t.Fatalf("resolve provider: %v", err)
		}
		if _, ok := p.(*claudeAdapter); !ok {
			t.Errorf("got %T, want *claudeAdapter", p)
		}
	})

	t.Run("groq without override and no platform client -> clear error", func(t *testing.T) {
		purpose := "test_purpose_groq_" + uuid.NewString()
		if err := repo.UpsertTaskConfig(ctx, purpose, ProviderGroq, groq.Model8B, "", userID); err != nil {
			t.Fatalf("upsert task config: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM ai_task_configs WHERE purpose = $1`, purpose) })

		if _, err := resolver.ResolveProvider(ctx, purpose); err == nil {
			t.Fatal("expected an error when groq has no platform client and no override key")
		}
	})

	t.Run("groq with override key returns a working groqAdapter", func(t *testing.T) {
		purpose := "test_purpose_groq_override_" + uuid.NewString()
		if err := repo.UpsertTaskConfig(ctx, purpose, ProviderGroq, groq.Model8B, "override-groq-key", userID); err != nil {
			t.Fatalf("upsert task config: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM ai_task_configs WHERE purpose = $1`, purpose) })

		p, err := resolver.ResolveProvider(ctx, purpose)
		if err != nil {
			t.Fatalf("resolve provider: %v", err)
		}
		if _, ok := p.(*groqAdapter); !ok {
			t.Errorf("got %T, want *groqAdapter", p)
		}
	})
}
