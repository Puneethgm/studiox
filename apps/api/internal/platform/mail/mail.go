// Package mail sends transactional email over SMTP. There is no
// queue/retry here, sends are synchronous and best-effort from the
// caller's perspective (a failed password-reset send should not reveal to
// the caller whether the recipient's account exists).
package mail

import (
	"fmt"
	htmlpkg "html"
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

// brandLogoURL is served by the production web app's public/ folder —
// email clients can't load a local file, so this has to be a real hosted
// URL, not an asset path. Shared by every email below so the signature
// block looks identical everywhere.
const brandLogoURL = "https://1herosocial.ai/logo.png"

// signatureHTML is the "Best regards" block every transactional email in
// this package ends with — small logo + "1herosocial.ai" + support email.
// A single shared block so a future brand tweak (new logo, new support
// address) only needs to change here, not in every Send* function.
func signatureHTML() string {
	return fmt.Sprintf(`
            <p style="margin:0 0 10px;font-size:13px;line-height:1.6;color:#52525b;">Best regards,</p>
            <table role="presentation" cellpadding="0" cellspacing="0"><tr>
              <td valign="middle" style="padding-right:10px;">
                <img src="%s" width="36" alt="1Hero Social" style="display:block;" />
              </td>
              <td valign="middle">
                <p style="margin:0;font-size:13px;font-weight:700;color:#18181b;">1herosocial.ai</p>
                <p style="margin:0;font-size:12px;color:#71717a;">support@1herosocial.ai</p>
              </td>
            </tr></table>`, brandLogoURL)
}

// SendPasswordReset emails a single-use reset link. The link itself already
// encodes the raw token (see identity's CreatePasswordResetToken) — this
// function only renders and delivers the message. userName and studioName
// are both optional (pass "" when unknown, e.g. a super-admin reset has no
// studio) — the copy adapts to whichever is present.
func (s *Sender) SendPasswordReset(toEmail, userName, studioName, resetLink string) error {
	greeting := "Hi there,"
	if userName != "" {
		greeting = fmt.Sprintf("Hi %s,", userName)
	}
	context := "You have requested to reset your password."
	if studioName != "" {
		context = fmt.Sprintf("You have requested to reset your password for <strong>%s</strong>.", studioName)
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Reset your password</h1>
          <p style="margin:0 0 8px;font-size:14px;line-height:1.6;color:#52525b;">%s</p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            %s Click the button below to choose a new one. This link can only be used once and expires in 1 hour.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Reset password
            </a>
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you didn't request this, you can safely ignore this email — your password won't change.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, greeting, context, resetLink, signatureHTML())

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
      <table role="presentation" width="680" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;overflow:hidden;">
        <tr>
          <!-- Left: message + login details -->
          <td width="400" valign="top" style="padding:40px 32px;">
            <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Welcome to 1Hero Social!</h1>
            <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
              We're excited to have you on board. <strong>%s</strong>'s account has been created and is ready for setup.
            </p>

            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f7;border-radius:12px;padding:16px 20px;margin:0 0 24px;">
              <tr><td style="font-size:11px;font-weight:800;letter-spacing:0.05em;text-transform:uppercase;color:#71717a;padding-bottom:10px;">Your Login Details</td></tr>
              <tr><td style="font-size:13px;line-height:2;color:#3f3f46;">
                Platform&nbsp;&nbsp;:&nbsp; <a href="%s" style="color:#7c3aed;text-decoration:none;">1herosocial.ai</a><br/>
                Email&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;:&nbsp; %s
              </td></tr>
            </table>

            <p style="margin:0 0 24px;">
              <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
                Set your password &amp; sign in
              </a>
            </p>
            <p style="margin:0 0 20px;font-size:12px;line-height:1.6;color:#a1a1aa;">
              That link is single-use and expires in 1 hour. Once you've set your password, sign in any time at
              <a href="%s" style="color:#7c3aed;">%s</a>.
            </p>

            %s
          </td>

          <!-- Right: brand panel -->
          <td width="280" valign="top" style="background:#f7f5fb;padding:32px 20px;text-align:center;">
            <img src="%s" width="220" alt="1Hero Social" style="display:block;margin:0 auto 20px;" />
            <p style="margin:0 0 8px;font-size:12px;color:#52525b;">support@1herosocial.ai</p>
            <p style="margin:0 0 20px;font-size:12px;color:#52525b;">1herosocial.ai</p>
            <p style="margin:0;font-size:13px;font-style:italic;color:#7c3aed;">Let's build your growth story.</p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, studioName, loginLink, toEmail, resetLink, loginLink, loginLink, signatureHTML(), brandLogoURL)

	return s.send(toEmail, "Welcome to 1herosocial.ai — set up your account", html)
}

// SendTeammateWelcome emails a newly-added studio teammate (identity's
// CreateStudioUserWithPassword) their login details right away. Unlike
// SendStudioWelcome/SendPasswordReset, this one shows the password in
// plaintext deliberately: it is a random one-time password generated for this
// teammate alone (identity.GenerateTempPassword), so the account is usable
// immediately without a round trip through a reset link. must_reset_password
// is still forced server-side, and the email says so, so the password stops
// working as soon as they choose their own.
func (s *Sender) SendTeammateWelcome(toEmail, studioName, roleName, username, password, loginLink string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Happy onboarding! 🎉</h1>
          <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
            You've been added to <strong>%s</strong> on 1Hero Social as <strong>%s</strong>.
          </p>

          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f7;border-radius:12px;padding:16px 20px;margin:0 0 24px;">
            <tr><td style="font-size:11px;font-weight:800;letter-spacing:0.05em;text-transform:uppercase;color:#71717a;padding-bottom:10px;">Your Login Details</td></tr>
            <tr><td style="font-size:13px;line-height:2;color:#3f3f46;">
              Platform&nbsp;&nbsp;:&nbsp; <a href="%s" style="color:#7c3aed;text-decoration:none;">1herosocial.ai</a><br/>
              Username&nbsp;:&nbsp; %s<br/>
              Email&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;:&nbsp; %s<br/>
              Password&nbsp;:&nbsp; %s
            </td></tr>
          </table>

          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Log in
            </a>
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            Once you've logged in, please reset your password from your account settings.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, studioName, roleName, loginLink, username, toEmail, password, loginLink, signatureHTML())

	return s.send(toEmail, fmt.Sprintf("Welcome to %s on 1herosocial.ai", studioName), html)
}

// EscalationMessageLine is one real message from the conversation that
// triggered the escalation, shown verbatim in the alert email — not an
// AI-generated summary, so the studio owner always sees exactly what was
// actually said regardless of whether AI is configured for that studio.
type EscalationMessageLine struct {
	FromLead bool // true = the lead said this; false = the studio/AI replied
	Body     string
}

// SendEscalationAlert notifies a studio the moment a conversation needs a
// human (see messaging.Service.EscalateAndNotify, the single call site for
// every escalation trigger). Best-effort — a failed send here must never
// block or fail the escalation itself, which is why this is always called
// from a goroutine by the caller, not inline.
func (s *Sender) SendEscalationAlert(toEmail, studioName, contactName, contactPhone, reason string, lines []EscalationMessageLine, escalationLink string) error {
	contactLabel := contactName
	if contactLabel == "" {
		contactLabel = contactPhone
	} else if contactPhone != "" {
		contactLabel = fmt.Sprintf("%s (%s)", contactName, contactPhone)
	}

	var transcript strings.Builder
	for _, l := range lines {
		who := "Studio"
		color := "#52525b"
		if l.FromLead {
			who = "Lead"
			color = "#18181b"
		}
		transcript.WriteString(fmt.Sprintf(`
            <p style="margin:0 0 8px;font-size:13px;line-height:1.5;">
              <strong style="color:%s;">%s:</strong>
              <span style="color:#3f3f46;">%s</span>
            </p>`, color, who, htmlpkg.EscapeString(l.Body)))
	}
	if len(lines) == 0 {
		transcript.WriteString(`<p style="margin:0;font-size:13px;color:#a1a1aa;">(no earlier messages in this conversation)</p>`)
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 8px;font-size:20px;font-weight:800;color:#18181b;">⚠️ A conversation needs you</h1>
          <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
            <strong>%s</strong> on <strong>%s</strong> was just escalated to a human.
          </p>

          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#fef2f2;border-radius:12px;padding:14px 18px;margin:0 0 20px;">
            <tr><td style="font-size:11px;font-weight:800;letter-spacing:0.05em;text-transform:uppercase;color:#b91c1c;padding-bottom:4px;">Why</td></tr>
            <tr><td style="font-size:13px;color:#7f1d1d;">%s</td></tr>
          </table>

          <div style="font-size:11px;font-weight:800;letter-spacing:0.05em;text-transform:uppercase;color:#71717a;margin:0 0 10px;">What was said</div>
          <div style="background:#f4f4f7;border-radius:12px;padding:16px 18px;margin:0 0 24px;">%s</div>

          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Open in Inbox
            </a>
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, htmlpkg.EscapeString(contactLabel), htmlpkg.EscapeString(studioName),
		htmlpkg.EscapeString(reason), transcript.String(), escalationLink, signatureHTML())

	return s.send(toEmail, fmt.Sprintf("⚠️ Escalation: %s — %s", studioName, reason), html)
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
