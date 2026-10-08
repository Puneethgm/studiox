// Command test-welcome-email sends a real transactional email to a given
// address, using live local SMTP config — for checking template changes
// land correctly without creating a throwaway studio/user in the DB.
//
// Usage: cd apps/api && go run ./cmd/test-welcome-email <toEmail> [welcome|reset|teammate]
// (kind defaults to "welcome")
package main

import (
	"fmt"
	"os"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/mail"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: test-welcome-email <toEmail> [welcome|reset|teammate]\n")
		os.Exit(1)
	}
	toEmail := os.Args[1]
	kind := "welcome"
	if len(os.Args) >= 3 {
		kind = os.Args[2]
	}

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	sender := mail.NewSender(cfg.SMTP)
	if !sender.Enabled() {
		os.Stderr.WriteString("SMTP not configured\n")
		os.Exit(1)
	}

	switch kind {
	case "welcome":
		err = sender.SendStudioWelcome(
			toEmail,
			"Demo Fitness Studio",
			"https://1herosocial.ai/login",
			"https://1herosocial.ai/reset-password?token=test-preview-token",
		)
	case "reset":
		err = sender.SendPasswordReset(
			toEmail,
			"Puneeth",
			"Demo Fitness Studio",
			"https://1herosocial.ai/reset-password?token=test-preview-token",
		)
	case "teammate":
		err = sender.SendTeammateWelcome(
			toEmail,
			"Demo Fitness Studio",
			"Front Desk",
			"puneeth.g",
			"Sample-temp-pw-Xk7q",
			"https://1herosocial.ai/login",
		)
	default:
		err = fmt.Errorf("unknown kind %q — use welcome, reset, or teammate", kind)
	}
	if err != nil {
		os.Stderr.WriteString("send failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	fmt.Println("sent " + kind + " email to " + toEmail)
}
