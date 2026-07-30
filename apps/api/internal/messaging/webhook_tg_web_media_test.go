package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestTGWebMedia_Integration verifies tg-web's media-upload endpoint saves
// real bytes under ./uploads (the part testable without a live MTProto
// session — the actual Telegram-side download only GramJS itself can do).
func TestTGWebMedia_Integration(t *testing.T) {
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		t.Fatalf("mkdir uploads: %v", err)
	}

	handler := NewHandler(nil, nil)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("mimeType", "image/jpeg")
	fw, err := mw.CreateFormFile("file", "photo.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	const fakeBytes = "fake-telegram-qr-photo-bytes"
	if _, err := fw.Write([]byte(fakeBytes)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/tg-web/media", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.tgWebMedia(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join("uploads", resp.Name)) })

	if resp.URL == "" || resp.Name == "" {
		t.Fatal("empty url/name in response")
	}
	diskBytes, err := os.ReadFile(filepath.Join("uploads", resp.Name))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(diskBytes) != fakeBytes {
		t.Errorf("saved bytes = %q, want %q", diskBytes, fakeBytes)
	}
	if filepath.Ext(resp.Name) != ".jpg" {
		t.Errorf("extension = %q, want .jpg (from filename)", filepath.Ext(resp.Name))
	}
}

// TestTGWebInbound_WithAttachment_Integration verifies the full path from
// the inbound webhook payload (as tg-web sends it once it has already
// uploaded media via tgWebMedia) through to a message row with a real
// attachment recorded.
func TestTGWebInbound_WithAttachment_Integration(t *testing.T) {
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

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "")
	handler := NewHandler(msgSvc, msgBus)

	if err := msgRepo.UpsertTGWebChannel(ctx, studioID, "+15550002222", "test_media_qr_user", "fake-session"); err != nil {
		t.Fatalf("seed tg-web channel: %v", err)
	}
	testChatID := "555222333"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (
			SELECT c.id FROM conversations c JOIN channel_accounts ca ON ca.id = c.channel_account_id
			WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto')`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (
			SELECT c.lead_id FROM conversations c JOIN channel_accounts ca ON ca.id = c.channel_account_id
			WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto')`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id IN (
			SELECT id FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto')`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM contact_identities WHERE studio_id = $1 AND kind = 'telegram_chat_id' AND value = $2`, studioID, testChatID)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto'`, studioID)
	})

	body, _ := json.Marshal(map[string]any{
		"studioId":       studioID.String(),
		"chatId":         testChatID,
		"text":           "",
		"messageId":      "tg-qr-media-1",
		"timestamp":      1234567890,
		"fromMe":         false,
		"attachmentUrl":  "/uploads/telegram-fake-upload.jpg",
		"attachmentType": "image",
		"attachmentMime": "image/jpeg",
		"attachmentName": "telegram-fake-upload.jpg",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/tg-web/inbound", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.tgWebInbound(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var attachmentsJSON []byte
	if err := pool.QueryRow(ctx, `
		SELECT m.attachments FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		JOIN channel_accounts ca ON ca.id = c.channel_account_id
		WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto'
	`, studioID).Scan(&attachmentsJSON); err != nil {
		t.Fatalf("query message: %v", err)
	}
	var atts []Attachment
	if err := json.Unmarshal(attachmentsJSON, &atts); err != nil {
		t.Fatalf("unmarshal attachments: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("attachments = %d, want 1", len(atts))
	}
	if atts[0].Type != "image" || atts[0].URL != "/uploads/telegram-fake-upload.jpg" {
		t.Errorf("attachment = %+v, want image at /uploads/telegram-fake-upload.jpg", atts[0])
	}
}
