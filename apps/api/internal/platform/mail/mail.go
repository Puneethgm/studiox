// Package mail sends transactional email over SMTP. Currently just
// password-reset links — there is no queue/retry here, sends are
// synchronous and best-effort from the caller's perspective (a failed send
// should not reveal to the caller whether the recipient's account exists).
package mail

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/projectx/api/internal/platform/config"
)

type Sender struct {
	cfg config.SMTPConfig
}

func NewSender(cfg config.SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) Enabled() bool { return s.cfg.Enabled() }

// SendPasswordReset emails a single-use reset link. The link itself already
// encodes the raw token (see identity's CreatePasswordResetToken) — this
// function only renders and delivers the message.
func (s *Sender) SendPasswordReset(toEmail, resetLink string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Reset your password</h1>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            We received a request to reset your password. Click the button below to choose a new one.
            This link can only be used once and expires in 1 hour.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Reset password
            </a>
          </p>
          <p style="margin:0;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you didn't request this, you can safely ignore this email — your password won't change.
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, resetLink)

	return s.send(toEmail, "Reset your password", html)
}

// SendStudioWelcome emails a brand-new studio's admin their onboarding
// message right after CreateStudioWithAdmin provisions the account with a
// default password — loginLink takes them straight to sign-in, resetLink
// (the same single-use, 1-hour token mechanism as SendPasswordReset) lets
// them set their own password immediately instead of ever typing the
// default one.
func (s *Sender) SendStudioWelcome(toEmail, studioName, loginLink, resetLink string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Congratulations on onboarding to 1herosocial.ai! 🎉</h1>
          <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
            <strong>%s</strong> is now live on the platform. We're excited to work with you and help you grow.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Set your password &amp; sign in
            </a>
          </p>
          <p style="margin:0 0 8px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            That link is single-use and expires in 1 hour. Once you've set your password, sign in any time at:
          </p>
          <p style="margin:0;font-size:12px;line-height:1.6;">
            <a href="%s" style="color:#7c3aed;">%s</a>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, studioName, resetLink, loginLink, loginLink)

	return s.send(toEmail, "Welcome to 1herosocial.ai — set up your account", html)
}

func (s *Sender) send(toEmail, subject, html string) error {
	if !s.cfg.Enabled() {
		return fmt.Errorf("smtp not configured")
	}

	msg := strings.Join([]string{
		"From: " + s.cfg.From,
		"To: " + toEmail,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"UTF-8\"",
		"",
		html,
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{toEmail}, []byte(msg))
}
