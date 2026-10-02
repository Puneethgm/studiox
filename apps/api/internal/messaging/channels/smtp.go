package channels

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// randomHex returns n random bytes hex-encoded, used for a unique Message-ID.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SMTPCredentials is what's stored (as JSON) in channel_accounts' encrypted
// access_token column for kind='email_smtp' — a studio's own outbound-email
// account, decoupled from the platform-wide SMTP config in
// internal/platform/mail (used only for password-reset/welcome emails).
type SMTPCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
}

// SMTPVerify dials the server and completes AUTH — no message is sent — to
// confirm the credentials actually work before they're stored. The email
// equivalent of TelegramGetMe.
func SMTPVerify(ctx context.Context, creds SMTPCredentials) error {
	client, err := dialSMTPAndAuth(ctx, creds)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Quit()
}

// dialSMTPAndAuth dials, STARTTLSes if offered, and authenticates — the part
// shared by SMTPVerify (which stops here) and SMTPSender (which goes on to
// send a message on the same connection).
func dialSMTPAndAuth(ctx context.Context, creds SMTPCredentials) (*smtp.Client, error) {
	if creds.Host == "" || creds.Port == 0 || creds.User == "" || creds.Password == "" || creds.From == "" {
		return nil, ErrInvalidCredentials
	}

	addr := net.JoinHostPort(creds.Host, strconv.Itoa(creds.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	// "tcp4": some hosts have a default IPv6 route that's configured but
	// doesn't actually route anywhere, so a dual-stack dial picks the IPv6
	// address and hangs for the full timeout instead of falling back to the
	// (working) IPv4 address. Every mainstream SMTP provider serves IPv4.
	conn, err := dialer.DialContext(ctx, "tcp4", addr)
	if err != nil {
		return nil, fmt.Errorf("dial smtp server: %w", err)
	}

	client, err := smtp.NewClient(conn, creds.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smtp handshake: %w", err)
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: creds.Host}); err != nil {
			client.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}

	auth := smtp.PlainAuth("", creds.User, creds.Password, creds.Host)
	if err := client.Auth(auth); err != nil {
		client.Close()
		return nil, ErrInvalidCredentials
	}
	return client, nil
}

// SMTPSender is the Sender implementation for kind='email_smtp' — a studio's
// own outbound-email account (see SMTPCredentials). Outbound-only: there's
// no inbox, so this never needs to parse inbound mail.
type SMTPSender struct{}

// SendText sends one plain-text email. accessToken is the JSON-encoded
// SMTPCredentials (decrypted by the caller); recipient is the destination
// email address. Attachments aren't sent as real MIME parts — their URLs
// are appended as plain links, matching how simple channels elsewhere in
// this package (e.g. SMS) pass media by reference rather than bytes.
func (s *SMTPSender) SendText(ctx context.Context, accessToken, _, recipient, subject, body string, attachments []Attachment) (*SendResult, error) {
	var creds SMTPCredentials
	if err := json.Unmarshal([]byte(accessToken), &creds); err != nil {
		return nil, fmt.Errorf("decode smtp credentials: %w", err)
	}

	client, err := dialSMTPAndAuth(ctx, creds)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Mail(creds.From); err != nil {
		return nil, fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return nil, fmt.Errorf("smtp RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return nil, fmt.Errorf("smtp DATA: %w", err)
	}

	if subject == "" {
		subject = "(no subject)"
	}
	messageID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), randomHex(8), creds.Host)

	fullBody := body
	for _, att := range attachments {
		if att.URL == "" {
			continue
		}
		fullBody += "\n\n" + att.URL
	}

	var msg strings.Builder
	msg.WriteString("From: " + creds.From + "\r\n")
	msg.WriteString("To: " + recipient + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("Message-ID: " + messageID + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(fullBody)

	if _, err := w.Write([]byte(msg.String())); err != nil {
		return nil, fmt.Errorf("smtp write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return nil, fmt.Errorf("smtp quit: %w", err)
	}

	return &SendResult{ExternalID: messageID}, nil
}
