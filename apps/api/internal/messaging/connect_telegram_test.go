package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

func TestConnectTelegramChannel_Integration(t *testing.T) {
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
	// t.Cleanup (not defer) — must outlive the row-cleanup t.Cleanup below,
	// which needs the pool still open. t.Cleanup runs LIFO, so registering
	// this first means it runs last.
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

	// Stub api.telegram.org for getMe + setWebhook.
	var sawSetWebhookURL string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/bottest-connect-token/getMe":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":777888999,"username":"connect_test_bot"}}`))
		case r.URL.Path == "/bottest-connect-token/setWebhook":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			sawSetWebhookURL, _ = body["url"].(string)
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer stub.Close()

	original := channels.TelegramAPIBaseURL
	channels.TelegramAPIBaseURL = stub.URL
	defer func() { channels.TelegramAPIBaseURL = original }()

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "https://studio.example.com")

	ch, err := msgSvc.ConnectTelegramChannel(ctx, studioID, ConnectTelegramInput{BotToken: "test-connect-token"})
	if err != nil {
		t.Fatalf("ConnectTelegramChannel: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, ch.ID)
	})

	if ch.Kind != KindTelegram {
		t.Errorf("Kind = %q, want telegram", ch.Kind)
	}
	if ch.ExternalID != "777888999" {
		t.Errorf("ExternalID = %q, want 777888999", ch.ExternalID)
	}
	if ch.DisplayHandle != "@connect_test_bot" {
		t.Errorf("DisplayHandle = %q", ch.DisplayHandle)
	}
	wantWebhookURL := "https://studio.example.com/api/v1/webhooks/telegram/777888999"
	if sawSetWebhookURL != wantWebhookURL {
		t.Errorf("setWebhook url = %q, want %q", sawSetWebhookURL, wantWebhookURL)
	}

	// Token must be encrypted at rest, not plaintext in the DB.
	var rawEnc string
	if err := pool.QueryRow(ctx, `SELECT access_token_enc FROM channel_accounts WHERE id = $1`, ch.ID).Scan(&rawEnc); err != nil {
		t.Fatalf("query access_token_enc: %v", err)
	}
	if rawEnc == "" {
		t.Fatal("access_token_enc is empty")
	}
	var leaked channels.TelegramCredentials
	if err := json.Unmarshal([]byte(rawEnc), &leaked); err == nil && leaked.BotToken == "test-connect-token" {
		t.Fatal("bot token stored in plaintext — access_token_enc must be encrypted")
	}
}
