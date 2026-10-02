// Command glofox-attendance-tick runs one attendance poll for a single
// studio right now, via the same Worker.TickStudio the CRM "connect" hook
// uses — without restarting the whole API server (which would also re-fire
// the firstsession worker's immediate-on-startup tick).
//
// Usage: cd apps/api && go run ./cmd/glofox-attendance-tick <studioId>
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/crm"
	"github.com/projectx/api/internal/integrations/glofox/attendance"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/logger"
	"github.com/projectx/api/internal/platform/secrets"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: glofox-attendance-tick <studioId>\n")
		os.Exit(1)
	}
	studioID, err := uuid.Parse(os.Args[1])
	if err != nil {
		os.Stderr.WriteString("invalid studio id: " + err.Error() + "\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	cipher, err := secrets.New(cfg.TokenEncryptionKey)
	if err != nil {
		log.Error("init cipher", "err", err)
		os.Exit(1)
	}

	crmRepo := crm.NewRepo(pool, cipher)
	attendanceRepo := attendance.NewRepo(pool)
	worker := attendance.New(crmRepo, attendanceRepo, log)

	if err := worker.TickStudio(ctx, studioID); err != nil {
		log.Error("tick studio", "studio_id", studioID, "err", err)
		os.Exit(1)
	}
	fmt.Println("done")
}
