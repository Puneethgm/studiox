// Command test-escalation exercises the real EscalateAndNotify code path
// (conversation repo flag + bus publish + studio-owner email) against a
// real studio/conversation in the local dev DB, without going through any
// channel-send logic — so it never messages the real contact, only emails
// the studio's contact_email.
//
// Usage: cd apps/api && go run ./cmd/test-escalation <studioId> <conversationId> [reason]
package main

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/projectx/api/internal/messaging"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/platform/mail"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

func main() {
	if len(os.Args) < 3 {
		os.Stderr.WriteString("usage: test-escalation <studioId> <conversationId> [reason]\n")
		os.Exit(1)
	}
	studioID, err := uuid.Parse(os.Args[1])
	if err != nil {
		os.Stderr.WriteString("bad studioId: " + err.Error() + "\n")
		os.Exit(1)
	}
	convID, err := uuid.Parse(os.Args[2])
	if err != nil {
		os.Stderr.WriteString("bad conversationId: " + err.Error() + "\n")
		os.Exit(1)
	}
	reason := "Test escalation — verifying studio-owner alert email"
	if len(os.Args) >= 4 {
		reason = os.Args[3]
	}

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB.DSN())
	if err != nil {
		os.Stderr.WriteString("db connect: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer pool.Close()

	cipher, err := secrets.New(cfg.TokenEncryptionKey)
	if err != nil {
		os.Stderr.WriteString("cipher: " + err.Error() + "\n")
		os.Exit(1)
	}

	studiosRepo := studios.NewRepo(pool, cipher)
	msgRepo := messaging.NewRepo(pool, cipher)
	msgBus := messaging.NewInProcBus()
	mailer := mail.NewSender(cfg.SMTP)
	if !mailer.Enabled() {
		os.Stderr.WriteString("SMTP not configured — email will not be attempted\n")
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	msgSvc := messaging.NewService(msgRepo, msgBus, cfg.PublicFormBaseURL, cfg.PublicAPIBaseURL, studiosRepo, mailer, frontendURL)

	if err := msgSvc.EscalateAndNotify(ctx, studioID, convID, reason); err != nil {
		os.Stderr.WriteString("escalate failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	os.Stdout.WriteString("conversation flagged escalated — waiting for the background email dispatch to finish...\n")
	// EscalateAndNotify fires the email send in its own goroutine; this
	// standalone binary has no other reason to stay alive, so just wait
	// long enough for a real SMTP round-trip before exiting.
	time.Sleep(8 * time.Second)
	os.Stdout.WriteString("done — check server logs (grep 'escalation alert email') and the studio's contact_email inbox.\n")
}
