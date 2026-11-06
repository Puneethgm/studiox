package mail

import (
	"testing"

	"github.com/projectx/api/internal/platform/config"
)

// TestSender_NewMethods_DegradeWhenSMTPNotConfigured covers platform-
// security-hardening task 1.7's two new transactional emails
// (SendNewDeviceLogin, SendSessionsEnded), confirming they fail the same
// best-effort way every other Send* method in this package already does
// when SMTP isn't configured — no network call attempted, no panic.
func TestSender_NewMethods_DegradeWhenSMTPNotConfigured(t *testing.T) {
	s := NewSender(config.SMTPConfig{}) // zero value: Enabled() is false
	if s.Enabled() {
		t.Fatal("expected a zero-value SMTPConfig to be disabled")
	}

	if err := s.SendNewDeviceLogin("user@example.com", "Jane", "Chrome on Mac", "Singapore", "Thu, 30 Oct 2026 12:00:00 UTC"); err == nil {
		t.Error("expected SendNewDeviceLogin to error when SMTP isn't configured")
	}
	if err := s.SendSessionsEnded("user@example.com", "Jane", "your password was changed"); err == nil {
		t.Error("expected SendSessionsEnded to error when SMTP isn't configured")
	}

	// Empty optional fields must not panic — same contract as the
	// existing SendPasswordReset's userName/studioName handling.
	if err := s.SendNewDeviceLogin("user@example.com", "", "", "", "Thu, 30 Oct 2026 12:00:00 UTC"); err == nil {
		t.Error("expected an error (SMTP not configured), got nil")
	}
	if err := s.SendSessionsEnded("user@example.com", "", ""); err == nil {
		t.Error("expected an error (SMTP not configured), got nil")
	}
}
