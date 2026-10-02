package channels

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer speaks just enough SMTP (EHLO, AUTH PLAIN, QUIT) to drive
// SMTPVerify without a real mail provider. It never advertises STARTTLS, so
// the client skips the TLS upgrade — PlainAuth still succeeds because
// isLocalhost(server.Name) is true for "127.0.0.1" (see net/smtp).
func fakeSMTPServer(t *testing.T, wantUser, wantPassword string) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		writeLine := func(s string) { conn.Write([]byte(s + "\r\n")) }
		writeLine("220 fake.smtp.test ESMTP ready")

		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "EHLO"):
				conn.Write([]byte("250-fake.smtp.test greets you\r\n250 AUTH PLAIN\r\n"))
			case strings.HasPrefix(line, "AUTH PLAIN "):
				payload, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
				parts := strings.Split(string(payload), "\x00")
				if decodeErr == nil && len(parts) == 3 && parts[1] == wantUser && parts[2] == wantPassword {
					writeLine("235 2.7.0 Authentication successful")
				} else {
					writeLine("535 5.7.8 Authentication failed")
				}
			case line == "*":
				writeLine("501 5.5.4 aborted")
			case strings.HasPrefix(line, "QUIT"):
				writeLine("221 2.0.0 Bye")
				return
			}
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func TestSMTPVerify_Success(t *testing.T) {
	host, port := fakeSMTPServer(t, "studio@example.com", "correct-password")

	err := SMTPVerify(context.Background(), SMTPCredentials{
		Host:     host,
		Port:     port,
		User:     "studio@example.com",
		Password: "correct-password",
		From:     "studio@example.com",
	})
	if err != nil {
		t.Fatalf("SMTPVerify() = %v, want nil", err)
	}
}

func TestSMTPVerify_WrongPassword(t *testing.T) {
	host, port := fakeSMTPServer(t, "studio@example.com", "correct-password")

	err := SMTPVerify(context.Background(), SMTPCredentials{
		Host:     host,
		Port:     port,
		User:     "studio@example.com",
		Password: "wrong-password",
		From:     "studio@example.com",
	})
	if err != ErrInvalidCredentials {
		t.Fatalf("SMTPVerify() = %v, want ErrInvalidCredentials", err)
	}
}

func TestSMTPVerify_MissingFields(t *testing.T) {
	err := SMTPVerify(context.Background(), SMTPCredentials{Host: "smtp.example.com"})
	if err != ErrInvalidCredentials {
		t.Fatalf("SMTPVerify() = %v, want ErrInvalidCredentials", err)
	}
}

func TestSMTPVerify_UnreachableServer(t *testing.T) {
	// A port nothing listens on — dial should fail with a wrapped error,
	// not ErrInvalidCredentials (that's reserved for rejected credentials).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // free the port so nothing answers

	err = SMTPVerify(context.Background(), SMTPCredentials{
		Host:     "127.0.0.1",
		Port:     port,
		User:     "studio@example.com",
		Password: "whatever",
		From:     "studio@example.com",
	})
	if err == nil || err == ErrInvalidCredentials {
		t.Fatalf("SMTPVerify() = %v, want a dial error", err)
	}
}
