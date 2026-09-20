package llm

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestEnabledModelsForStudio_Integration confirms the fallback-to-default
// behavior (internal/studios/ai_models_http.go's GET endpoint and
// ai_worker.go's waterfall both depend on this): a studio that never
// configured a provider, or disabled every model it added, still gets a
// usable model list rather than going dark. Same DB-test shape as
// resolve_test.go — real local Postgres, t.Skip if unconfigured.
func TestEnabledModelsForStudio_Integration(t *testing.T) {
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

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio found in DB")
	}

	t.Run("zero rows falls back to DefaultModels", func(t *testing.T) {
		models, err := repo.EnabledModelsForStudio(ctx, studioID, ProviderGroq)
		if err != nil {
			t.Fatalf("enabled models: %v", err)
		}
		if len(models) != len(DefaultModels[ProviderGroq]) {
			t.Fatalf("got %v, want default fallback %v", models, DefaultModels[ProviderGroq])
		}
	})

	t.Run("an enabled row wins over the default", func(t *testing.T) {
		const modelName = "test-model-enabled"
		if _, err := repo.UpsertStudioModelResult(ctx, studioID, ProviderGroq, modelName, true, ""); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM studio_ai_models WHERE studio_id = $1 AND provider = $2`, studioID, ProviderGroq)
		})

		models, err := repo.EnabledModelsForStudio(ctx, studioID, ProviderGroq)
		if err != nil {
			t.Fatalf("enabled models: %v", err)
		}
		if len(models) != 1 || models[0] != modelName {
			t.Fatalf("got %v, want [%s]", models, modelName)
		}
	})

	t.Run("all rows disabled falls back to DefaultModels too", func(t *testing.T) {
		const modelName = "test-model-disabled"
		row, err := repo.UpsertStudioModelResult(ctx, studioID, ProviderClaude, modelName, true, "")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM studio_ai_models WHERE studio_id = $1 AND provider = $2`, studioID, ProviderClaude)
		})
		if err := repo.SetStudioModelEnabled(ctx, studioID, row.ID, false); err != nil {
			t.Fatalf("disable: %v", err)
		}

		models, err := repo.EnabledModelsForStudio(ctx, studioID, ProviderClaude)
		if err != nil {
			t.Fatalf("enabled models: %v", err)
		}
		if len(models) != len(DefaultModels[ProviderClaude]) {
			t.Fatalf("got %v, want default fallback %v", models, DefaultModels[ProviderClaude])
		}
	})

	t.Run("upsert on an existing model_name updates in place, no duplicate", func(t *testing.T) {
		const modelName = "test-model-upsert"
		if _, err := repo.UpsertStudioModelResult(ctx, studioID, ProviderGemini, modelName, false, "first attempt failed"); err != nil {
			t.Fatalf("first upsert: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM studio_ai_models WHERE studio_id = $1 AND provider = $2`, studioID, ProviderGemini)
		})
		if _, err := repo.UpsertStudioModelResult(ctx, studioID, ProviderGemini, modelName, true, ""); err != nil {
			t.Fatalf("second upsert: %v", err)
		}

		rows, err := repo.ListStudioModels(ctx, studioID, ProviderGemini)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("got %d rows, want 1 (no duplicate on repeated upsert)", len(rows))
		}
		if !rows[0].Enabled || rows[0].LastTestOK == nil || !*rows[0].LastTestOK {
			t.Fatalf("expected the second (successful) test result to win, got %+v", rows[0])
		}
	})
}
