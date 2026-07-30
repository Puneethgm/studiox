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

// TestTGWebConnected_Integration exercises the internal receiver tg-web
// calls on a successful QR login (tgWebConnected), and confirms:
//   - the session string round-trips through ListTGWebSessions (used to
//     rehydrate tg-web after a restart) exactly as stored
//   - it's encrypted at rest, not sitting in the DB as plaintext
//   - a disconnect makes it disappear from ListTGWebSessions (only 'active'
//     channels are eligible for rehydration)
func TestTGWebConnected_Integration(t *testing.T) {
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

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto'`, studioID)
	})

	sessionString := "1BVtsOK4Bu-fake-gramjs-session-string-abc123"
	body, _ := json.Marshal(map[string]any{
		"studioId":      studioID.String(),
		"phone":         "+15550009999",
		"username":      "test_qr_user",
		"sessionString": sessionString,
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/tg-web/connected", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.tgWebConnected(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("connected status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Token must be encrypted at rest.
	var rawEnc string
	if err := pool.QueryRow(ctx, `SELECT access_token_enc FROM channel_accounts WHERE studio_id = $1 AND kind = 'telegram_mtproto'`, studioID).Scan(&rawEnc); err != nil {
		t.Fatalf("query access_token_enc: %v", err)
	}
	if rawEnc == sessionString {
		t.Fatal("session string stored in plaintext — access_token_enc must be encrypted")
	}

	// Round-trips correctly through ListTGWebSessions (what tg-web uses to
	// rehydrate itself after a restart).
	sessions, err := msgRepo.ListTGWebSessions(ctx)
	if err != nil {
		t.Fatalf("ListTGWebSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.StudioID == studioID {
			found = true
			if s.SessionString != sessionString {
				t.Fatalf("decrypted session string = %q, want %q", s.SessionString, sessionString)
			}
		}
	}
	if !found {
		t.Fatal("studio's session not present in ListTGWebSessions")
	}

	// Disconnecting removes it from the rehydration set.
	if err := msgRepo.DisconnectTGWebChannel(ctx, studioID); err != nil {
		t.Fatalf("DisconnectTGWebChannel: %v", err)
	}
	sessionsAfter, err := msgRepo.ListTGWebSessions(ctx)
	if err != nil {
		t.Fatalf("ListTGWebSessions after disconnect: %v", err)
	}
	for _, s := range sessionsAfter {
		if s.StudioID == studioID {
			t.Fatal("disconnected channel still present in ListTGWebSessions")
		}
	}
}
