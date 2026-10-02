package messaging

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

// fakeSMTPServer speaks just enough SMTP (EHLO, AUTH PLAIN, QUIT) to drive
// ConnectEmailSMTPChannel's verify step, mirroring channels/smtp_test.go's
// double.
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

func TestConnectEmailSMTPChannel_Integration(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio found in DB")
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}

	host, port := fakeSMTPServer(t, "studio@example.com", "correct-password")

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "https://studio.example.com", nil, nil, "")

	ch, err := msgSvc.ConnectEmailSMTPChannel(ctx, studioID, ConnectEmailSMTPInput{
		Host:     host,
		Port:     port,
		User:     "studio@example.com",
		Password: "correct-password",
		From:     "studio@example.com",
	})
	if err != nil {
		t.Fatalf("ConnectEmailSMTPChannel: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, ch.ID)
	})

	if ch.Kind != KindEmailSMTP {
		t.Errorf("Kind = %q, want email_smtp", ch.Kind)
	}
	if ch.DisplayHandle != "studio@example.com" {
		t.Errorf("DisplayHandle = %q", ch.DisplayHandle)
	}

	// Credentials must be encrypted at rest, not plaintext in the DB.
	var rawEnc string
	if err := pool.QueryRow(ctx, `SELECT access_token_enc FROM channel_accounts WHERE id = $1`, ch.ID).Scan(&rawEnc); err != nil {
		t.Fatalf("query access_token_enc: %v", err)
	}
	if rawEnc == "" {
		t.Fatal("access_token_enc is empty")
	}
	var leaked channels.SMTPCredentials
	if err := json.Unmarshal([]byte(rawEnc), &leaked); err == nil && leaked.Password == "correct-password" {
		t.Fatal("SMTP password stored in plaintext — access_token_enc must be encrypted")
	}

	// GetActiveEmailSMTPCredentials should round-trip the decrypted creds.
	got, err := msgSvc.GetActiveEmailSMTPCredentials(ctx, studioID)
	if err != nil {
		t.Fatalf("GetActiveEmailSMTPCredentials: %v", err)
	}
	if got.Host != host || got.Port != port || got.User != "studio@example.com" || got.Password != "correct-password" {
		t.Errorf("GetActiveEmailSMTPCredentials() = %+v, want matching host/port/user/password", got)
	}
}

func TestConnectEmailSMTPChannel_InvalidCredentials(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio found in DB")
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}

	host, port := fakeSMTPServer(t, "studio@example.com", "correct-password")

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "https://studio.example.com", nil, nil, "")

	_, err = msgSvc.ConnectEmailSMTPChannel(ctx, studioID, ConnectEmailSMTPInput{
		Host:     host,
		Port:     port,
		User:     "studio@example.com",
		Password: "wrong-password",
		From:     "studio@example.com",
	})
	if err == nil {
		t.Fatal("ConnectEmailSMTPChannel: want error for wrong password, got nil")
	}
}
