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
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

// TestTelegramWebhook_InboundPhoto_Integration verifies a photo message
// gets downloaded via Telegram's getFile + file-download flow and stored
// as a real attachment on the message row — not just parsed and dropped.
func TestTelegramWebhook_InboundPhoto_Integration(t *testing.T) {
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

	// Run relative to this package dir, so ./uploads matches what the
	// download function (and every other channel's inbound media path)
	// actually writes to at runtime.
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		t.Fatalf("mkdir uploads: %v", err)
	}

	const fakeImageBytes = "fake-jpeg-bytes-from-telegram"
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/bottest-media-token/getFile":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_path":"photos/file_1.jpg"}}`))
		case r.URL.Path == "/file/bottest-media-token/photos/file_1.jpg":
			_, _ = w.Write([]byte(fakeImageBytes))
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
	msgSvc := NewService(msgRepo, msgBus, "", "")
	handler := NewTelegramWebhookHandler(msgSvc, slog.Default())

	botExternalID := fmt.Sprintf("test-media-bot-%s", uuid.NewString()[:8])
	secretToken := "test-secret"
	creds, _ := json.Marshal(channels.TelegramCredentials{
		BotToken:      "test-media-token",
		WebhookSecret: secretToken,
	})
	channel, err := msgRepo.CreateChannel(ctx, CreateChannelInput{
		StudioID:      studioID,
		Kind:          KindTelegram,
		BSP:           "telegram",
		ExternalID:    botExternalID,
		DisplayHandle: "@test_media_bot",
		AccessToken:   string(creds),
	})
	if err != nil {
		t.Fatalf("create test channel: %v", err)
	}

	var downloadedFileName string
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (SELECT lead_id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id = $1`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, channel.ID)
		if downloadedFileName != "" {
			_ = os.Remove(filepath.Join("uploads", downloadedFileName))
		}
	})

	update := channels.TelegramUpdate{
		UpdateID: 1,
		Message: &channels.TelegramMessage{
			MessageID: 555,
			Date:      1234567890,
			Caption:   "check out this photo",
			Chat:      channels.TelegramChat{ID: 777000111, Type: "private"},
			From:      &channels.TelegramUser{ID: 777000111, FirstName: "Ada"},
			Photo: []channels.TelegramPhotoSize{
				{FileID: "small-file-id", Width: 90, Height: 90},
				{FileID: "large-file-id", Width: 800, Height: 800}, // largest — should be the one downloaded
			},
		},
	}
	body, _ := json.Marshal(update)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram/"+botExternalID, bytes.NewReader(body))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secretToken)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("botID", botExternalID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.HandleInbound(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var msgBody string
	var attachmentsJSON []byte
	if err := pool.QueryRow(ctx, `
		SELECT m.body, m.attachments
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.channel_account_id = $1
	`, channel.ID).Scan(&msgBody, &attachmentsJSON); err != nil {
		t.Fatalf("query message: %v", err)
	}
	if msgBody != "check out this photo" {
		t.Errorf("body = %q, want caption to be used as body", msgBody)
	}

	var atts []Attachment
	if err := json.Unmarshal(attachmentsJSON, &atts); err != nil {
		t.Fatalf("unmarshal attachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1", len(atts))
	}
	if atts[0].Type != "image" {
		t.Errorf("attachment type = %q, want image", atts[0].Type)
	}
	if atts[0].URL == "" {
		t.Fatal("attachment URL is empty — download must have failed")
	}

	downloadedFileName = filepath.Base(atts[0].URL)
	diskBytes, err := os.ReadFile(filepath.Join("uploads", downloadedFileName))
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(diskBytes) != fakeImageBytes {
		t.Errorf("downloaded file contents = %q, want %q (the LARGEST photo size's bytes, not a placeholder)", diskBytes, fakeImageBytes)
	}
}
