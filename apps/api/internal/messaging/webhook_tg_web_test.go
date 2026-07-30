package messaging

import (
	"bytes"
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

	"github.com/projectx/api/internal/platform/secrets"
)

func TestTGWebInboundAndBackfill_Integration(t *testing.T) {
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

	// Seed a "connected" tg-web session the way tg-web's /internal/tg-web/connected
	// receiver would (see tgWebConnected).
	testChatID := "555111222"
	if err := msgRepo.UpsertTGWebChannel(ctx, studioID, "+15550001111", "test_tg_user", "fake-session-string"); err != nil {
		t.Fatalf("seed tg-web channel: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (
			SELECT c.id FROM conversations c
			JOIN channel_accounts ca ON ca.id = c.channel_account_id
			WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto'
		)`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (
			SELECT c.lead_id FROM conversations c
			JOIN channel_accounts ca ON ca.id = c.channel_account_id
			WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto'
		)`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id IN (
			SELECT id FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto'
		)`, studioID)
		_, _ = pool.Exec(ctx, `DELETE FROM contact_identities WHERE studio_id = $1 AND kind = 'telegram_chat_id' AND value = $2`, studioID, testChatID)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto'`, studioID)
	})

	postInbound := func(messageID string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"studioId":  studioID.String(),
			"chatId":    testChatID,
			"text":      "Hi, interested in a trial class",
			"messageId": messageID,
			"timestamp": 1234567890,
			"fromMe":    false,
		})
		req := httptest.NewRequest(http.MethodPost, "/internal/tg-web/inbound", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.tgWebInbound(rec, req)
		return rec
	}

	// --- first delivery creates conversation + message ---
	rec := postInbound("tg-msg-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var convID uuid.UUID
	var msgCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.id, (SELECT count(*) FROM messages m WHERE m.conversation_id = c.id)
		FROM conversations c
		JOIN channel_accounts ca ON ca.id = c.channel_account_id
		WHERE ca.studio_id = $1 AND ca.kind = 'telegram_mtproto'
	`, studioID).Scan(&convID, &msgCount); err != nil {
		t.Fatalf("query conversation: %v", err)
	}
	if msgCount != 1 {
		t.Fatalf("message count after first delivery = %d, want 1", msgCount)
	}

	// --- retried delivery (same messageId) doesn't duplicate ---
	rec2 := postInbound("tg-msg-1")
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec2.Code, rec2.Body.String())
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE conversation_id = $1`, convID).Scan(&msgCount); err != nil {
		t.Fatalf("query messages after retry: %v", err)
	}
	if msgCount != 1 {
		t.Fatalf("message count after retried delivery = %d, want 1 (idempotency broken)", msgCount)
	}

	// --- backfill imports historical messages without bumping unread/last-message ---
	var unreadBefore int
	if err := pool.QueryRow(ctx, `SELECT unread_count FROM conversations WHERE id = $1`, convID).Scan(&unreadBefore); err != nil {
		t.Fatalf("query unread before backfill: %v", err)
	}

	backfillBody, _ := json.Marshal(map[string]any{
		"studioId":    studioID.String(),
		"displayName": "Ada Test",
		"messages": []map[string]any{
			{"chatId": testChatID, "text": "old message 1", "messageId": "tg-hist-1", "timestamp": 1000000000, "fromMe": false},
			{"chatId": testChatID, "text": "old message 2", "messageId": "tg-hist-2", "timestamp": 1000000100, "fromMe": true},
		},
	})
	backfillReq := httptest.NewRequest(http.MethodPost, "/internal/tg-web/backfill", bytes.NewReader(backfillBody))
	backfillRec := httptest.NewRecorder()
	handler.tgWebBackfill(backfillRec, backfillReq)
	if backfillRec.Code != http.StatusOK {
		t.Fatalf("backfill status = %d, body = %s", backfillRec.Code, backfillRec.Body.String())
	}
	var backfillResp struct {
		Imported int `json:"imported"`
	}
	if err := json.Unmarshal(backfillRec.Body.Bytes(), &backfillResp); err != nil {
		t.Fatalf("decode backfill response: %v", err)
	}
	if backfillResp.Imported != 2 {
		t.Fatalf("imported = %d, want 2", backfillResp.Imported)
	}

	var unreadAfter int
	if err := pool.QueryRow(ctx, `SELECT unread_count FROM conversations WHERE id = $1`, convID).Scan(&unreadAfter); err != nil {
		t.Fatalf("query unread after backfill: %v", err)
	}
	if unreadAfter != unreadBefore {
		t.Fatalf("backfill must not bump unread_count: before=%d after=%d", unreadBefore, unreadAfter)
	}

	// --- re-running the same backfill batch is idempotent ---
	backfillRec2 := httptest.NewRecorder()
	handler.tgWebBackfill(backfillRec2, httptest.NewRequest(http.MethodPost, "/internal/tg-web/backfill", bytes.NewReader(backfillBody)))
	var backfillResp2 struct {
		Imported int `json:"imported"`
	}
	_ = json.Unmarshal(backfillRec2.Body.Bytes(), &backfillResp2)
	if backfillResp2.Imported != 0 {
		t.Fatalf("re-running backfill imported = %d, want 0 (idempotency broken)", backfillResp2.Imported)
	}
}
