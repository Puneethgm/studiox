package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

func TestTelegramWebhook_Integration(t *testing.T) {
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

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "")
	handler := NewTelegramWebhookHandler(msgSvc, slog.Default())

	botExternalID := fmt.Sprintf("test-bot-%s", uuid.NewString()[:8])
	secretToken := "test-secret-token"
	creds, _ := json.Marshal(channels.TelegramCredentials{
		BotToken:      "123456:fake-token-for-test",
		WebhookSecret: secretToken,
	})

	channel, err := msgRepo.CreateChannel(ctx, CreateChannelInput{
		StudioID:      studioID,
		Kind:          KindTelegram,
		BSP:           "telegram",
		ExternalID:    botExternalID,
		DisplayHandle: "@test_studio_bot",
		AccessToken:   string(creds),
	})
	if err != nil {
		t.Fatalf("create test channel: %v", err)
	}
	t.Cleanup(func() {
		// Manual cleanup in FK-safe order — channel_accounts is RESTRICT'd by conversations.
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (SELECT lead_id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id = $1`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM contact_identities WHERE studio_id = $1 AND kind = 'telegram_chat_id' AND value = $2`, studioID, "555000111")
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, channel.ID)
	})

	update := channels.TelegramUpdate{
		UpdateID: 1,
		Message: &channels.TelegramMessage{
			MessageID: 42,
			Date:      1234567890,
			Text:      "Hi, I want to try a class",
			Chat:      channels.TelegramChat{ID: 555000111, Type: "private"},
			From:      &channels.TelegramUser{ID: 555000111, FirstName: "Ada", Username: "ada_test"},
		},
	}
	body, _ := json.Marshal(update)

	postUpdate := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram/"+botExternalID, bytes.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secretToken)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("botID", botExternalID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		rec := httptest.NewRecorder()
		handler.HandleInbound(rec, req)
		return rec
	}

	// --- wrong secret is rejected ---
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram/"+botExternalID, bytes.NewReader(body))
	badReq.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("botID", botExternalID)
	badReq = badReq.WithContext(context.WithValue(badReq.Context(), chi.RouteCtxKey, rctx))
	badRec := httptest.NewRecorder()
	handler.HandleInbound(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("wrong secret: status = %d, want 403", badRec.Code)
	}

	// --- first delivery creates conversation + message ---
	rec := postUpdate()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var convID uuid.UUID
	var msgCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.id, (SELECT count(*) FROM messages m WHERE m.conversation_id = c.id)
		FROM conversations c
		WHERE c.channel_account_id = $1
	`, channel.ID).Scan(&convID, &msgCount); err != nil {
		t.Fatalf("query conversation: %v", err)
	}
	if msgCount != 1 {
		t.Fatalf("message count after first delivery = %d, want 1", msgCount)
	}

	// --- retried delivery (Telegram redelivery on timeout) doesn't duplicate ---
	rec2 := postUpdate()
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec2.Code, rec2.Body.String())
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE conversation_id = $1`, convID).Scan(&msgCount); err != nil {
		t.Fatalf("query messages after retry: %v", err)
	}
	if msgCount != 1 {
		t.Fatalf("message count after retried delivery = %d, want 1 (idempotency broken)", msgCount)
	}
}
