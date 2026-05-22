package main

import (
	"context"
	"os"
	"time"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	if cfg.SuperUser.Email == "" || cfg.SuperUser.Password == "" {
		log.Error("SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD must be set in .env")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, err := identity.HashPassword(cfg.SuperUser.Password)
	if err != nil {
		log.Error("hash password", "err", err)
		os.Exit(1)
	}

	repo := identity.NewRepo(pool)
	id, err := repo.UpsertSuperAdmin(ctx, cfg.SuperUser.Email, hash)
	if err != nil {
		log.Error("upsert super admin", "err", err)
		os.Exit(1)
	}
	log.Info("super admin ready", "id", id, "email", cfg.SuperUser.Email)
}
