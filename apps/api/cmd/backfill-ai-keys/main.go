// Command backfill-ai-keys is a one-off, operator-run step for the
// platform-key-store migration (see
// openspec/changes/llm-gateway-and-key-management, task 2.5): it moves any
// provider key still sitting in the old platform_settings storage
// (single-static-key AES) into the new envelope-encrypted platform_ai_keys
// table. Idempotent — safe to re-run; a provider already migrated is left
// untouched.
//
// Usage: cd apps/api && go run ./cmd/backfill-ai-keys
package main

import (
	"context"
	"os"
	"time"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	if cfg.TokenEncryptionKey == "" {
		log.Error("TOKEN_ENCRYPTION_KEY must be set to migrate platform AI provider keys")
		os.Exit(1)
	}
	cipher, err := secrets.New(cfg.TokenEncryptionKey)
	if err != nil {
		log.Error("init cipher", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := studios.NewRepo(pool, cipher)
	migrated, err := repo.MigrateLegacyAIProviderKeys(ctx)
	if err != nil {
		log.Error("migrate legacy ai provider keys", "err", err)
		os.Exit(1)
	}
	if len(migrated) == 0 {
		log.Info("no legacy platform AI provider keys to migrate")
		return
	}
	log.Info("migrated legacy platform AI provider keys to envelope-encrypted store", "providers", migrated)
}
