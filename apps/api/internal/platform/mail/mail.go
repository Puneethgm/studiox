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

// SendAccountDeletionPending emails a studio's registered contact right
// after an account-deletion request is made — the "pre-deletion email"
// step of init.md §4.5. It explains the account is about to be permanently
// deleted, that studio data will stop being reachable through the active
// account once that happens, and that confirmLink must be clicked to
// actually proceed. The link is single-use (the confirm token is consumed
// on use) and expires after 24 hours.
func (s *Sender) SendAccountDeletionPending(toEmail, studioName, confirmLink string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Confirm account deletion</h1>
          <p style="margin:0 0 8px;font-size:14px;line-height:1.6;color:#52525b;">
            A request was made to permanently delete <strong>%s</strong>'s account on 1Hero Social.
          </p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            Once confirmed, the account and its data will be permanently deleted and will no longer be
            available through the active account. Click the button below to confirm. This link can only
            be used once and expires in 24 hours.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#dc2626;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Confirm permanent deletion
            </a>
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you didn't request this, you can safely ignore this email — the account will not be deleted
            unless this link is clicked.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, htmlpkg.EscapeString(studioName), confirmLink, signatureHTML())

	return s.send(toEmail, fmt.Sprintf("Confirm deletion of %s's account", studioName), html)
}

// SendAccountDeletedConfirmation emails a studio's registered contact once
// account deletion has actually completed — the "post-deletion
// confirmation email" step of init.md §4.5. It confirms the account is
// gone and that the encrypted data archive taken just before deletion can
// be recovered/downloaded via recoveryLink within 7 days, after which the
// link expires permanently and the archive can no longer be recovered.
func (s *Sender) SendAccountDeletedConfirmation(toEmail, studioName, recoveryLink string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Account deleted</h1>
          <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
            <strong>%s</strong>'s account on 1Hero Social has been permanently deleted, as requested.
          </p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            An encrypted backup of the account's data was made before deletion. You can recover or
            download it using the link below within the next 7 days — after that, the link expires
            permanently and the backup can no longer be recovered.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Recover my data
            </a>
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you didn't request this deletion, contact support immediately using the details below.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, htmlpkg.EscapeString(studioName), recoveryLink, signatureHTML())

	return s.send(toEmail, fmt.Sprintf("%s's account has been deleted", studioName), html)
}

// SendNewDeviceLogin alerts a user that their account just signed in
// from a device/browser not seen before for that account — platform-
// security-hardening task 1.7. device/location are best-effort,
// human-readable strings (e.g. a parsed User-Agent, an IP or city); pass
// "" for either when unknown and the copy adapts.
func (s *Sender) SendNewDeviceLogin(toEmail, userName, device, location, when string) error {
	greeting := "Hi there,"
	if userName != "" {
		greeting = fmt.Sprintf("Hi %s,", userName)
	}
	deviceLine := "a new device"
	if device != "" {
		deviceLine = htmlpkg.EscapeString(device)
	}
	locationLine := ""
	if location != "" {
		locationLine = fmt.Sprintf(" from %s", htmlpkg.EscapeString(location))
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">New sign-in to your account</h1>
          <p style="margin:0 0 8px;font-size:14px;line-height:1.6;color:#52525b;">%s</p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            Your account was just signed into from %s%s at %s. If this was you, no action is needed.
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you don't recognize this, change your password immediately and end your other sessions from Settings.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, greeting, deviceLine, locationLine, htmlpkg.EscapeString(when), signatureHTML())

	return s.send(toEmail, "New sign-in to your account", html)
}

// SendSessionsEnded tells a user every one of their sessions was just
// ended — by an admin's force-logout, or a platform-wide "end all
// sessions" (task 1.7). reason is a short, human-readable cause (e.g.
// "your password was changed" or "an administrator ended your sessions").
func (s *Sender) SendSessionsEnded(toEmail, userName, reason string) error {
	greeting := "Hi there,"
	if userName != "" {
		greeting = fmt.Sprintf("Hi %s,", userName)
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">You've been signed out everywhere</h1>
          <p style="margin:0 0 8px;font-size:14px;line-height:1.6;color:#52525b;">%s</p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            All of your active sessions were just ended (%s). You'll need to sign in again on each device.
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If you didn't expect this, contact your administrator.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, greeting, htmlpkg.EscapeString(reason), signatureHTML())

	return s.send(toEmail, "You've been signed out everywhere", html)
}

// SendTwoFactorLockout alerts a user that their account's code step was
// just locked for 15 minutes after 5 consecutive wrong authenticator
// codes — platform-security-hardening task 2.4. This is a real signal
// someone else may be trying to get in (they already had the correct
// password), so the copy is a notice, not a password-reset nag.
func (s *Sender) SendTwoFactorLockout(toEmail, userName string) error {
	greeting := "Hi there,"
	if userName != "" {
		greeting = fmt.Sprintf("Hi %s,", userName)
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">Too many wrong codes</h1>
          <p style="margin:0 0 8px;font-size:14px;line-height:1.6;color:#52525b;">%s</p>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">
            Someone entered 5 wrong authenticator codes in a row for your account (the password was correct). Sign-in with a code is locked for 15 minutes.
          </p>
          <p style="margin:0 0 24px;font-size:12px;line-height:1.6;color:#a1a1aa;">
            If this wasn't you, consider changing your password — whoever tried already had it right.
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, greeting, signatureHTML())

	return s.send(toEmail, "Too many wrong codes on your account", html)
}

// SendSecurityAlert is the generic platform-team notification behind
// platform-security-hardening task 3.7's risk alerts (new-country admin
// login, 2FA disabled, secret changed, data exported, admin role
// granted, 5+ failed logins, admin action outside studio hours) — one
// template rather than seven, since every one of these is "here is what
// happened, to whom, when" with no further structure worth a dedicated
// layout. detail is a short plain-text line describing the specific
// trigger.
func (s *Sender) SendSecurityAlert(toEmail, title, detail string) error {
	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 16px;font-size:20px;font-weight:800;color:#18181b;">%s</h1>
          <p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#52525b;">%s</p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, title, detail, signatureHTML())

	return s.send(toEmail, "Security alert: "+title, html)
}

// FollowupDigestItem is one due reminder shown in SendFollowupDigest.
type FollowupDigestItem struct {
	LeadName string
	Note     string // reminder_note, may be empty
	LeadLink string // deep link to open this lead in the dashboard
}

// SendFollowupDigest emails a studio a once-daily list of leads whose
// manual follow-up reminder (see leads.reminder_at/reminder_note) is now
// due — see FollowupDigestWorker, the only caller. Never sent with an
// empty items list (the worker skips studios with nothing due that day).
func (s *Sender) SendFollowupDigest(toEmail, studioName string, items []FollowupDigestItem, leadsLink string) error {
	var rows strings.Builder
	for _, it := range items {
		note := `<span style="color:#a1a1aa;">(no note)</span>`
		if it.Note != "" {
			note = htmlpkg.EscapeString(it.Note)
		}
		rows.WriteString(fmt.Sprintf(`
            <tr>
              <td style="padding:10px 0;border-top:1px solid #e4e4e7;">
                <a href="%s" style="color:#18181b;font-weight:700;font-size:14px;text-decoration:none;">%s</a>
                <p style="margin:4px 0 0;font-size:13px;color:#52525b;">%s</p>
              </td>
            </tr>`, it.LeadLink, htmlpkg.EscapeString(it.LeadName), note))
	}

	html := fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table role="presentation" width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:16px;padding:40px;">
        <tr><td>
          <h1 style="margin:0 0 8px;font-size:20px;font-weight:800;color:#18181b;">📋 Today's follow-ups</h1>
          <p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#52525b;">
            <strong>%d</strong> lead%s at <strong>%s</strong> %s due for a follow-up.
          </p>

          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin:0 0 24px;">%s</table>

          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#7c3aed;color:#ffffff;text-decoration:none;font-weight:700;font-size:14px;padding:12px 24px;border-radius:12px;">
              Open Leads
            </a>
          </p>
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, len(items), pluralSuffix(len(items)), htmlpkg.EscapeString(studioName), isAreSuffix(len(items)), rows.String(), leadsLink, signatureHTML())

	subject := fmt.Sprintf("📋 %d follow-up%s due today — %s", len(items), pluralSuffix(len(items)), studioName)
	return s.send(toEmail, subject, html)
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func isAreSuffix(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
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
