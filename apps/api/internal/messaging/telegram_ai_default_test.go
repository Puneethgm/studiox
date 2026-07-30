package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/messaging/channels"
	"github.com/projectx/api/internal/platform/secrets"
)

// TestTelegramBot_AIEnabledByDefault_ButNotForcedBackOn verifies that a new
// Telegram bot conversation starts with AI auto-reply already on (unlike
// every other channel, which defaults off), but a staff member turning it
// off afterward is never silently re-enabled by a later inbound message.
func TestTelegramBot_AIEnabledByDefault_ButNotForcedBackOn(t *testing.T) {
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

	botExternalID := fmt.Sprintf("test-ai-default-bot-%s", uuid.NewString()[:8])
	creds, _ := json.Marshal(channels.TelegramCredentials{BotToken: "fake-token", WebhookSecret: "shh"})
	channel, err := msgRepo.CreateChannel(ctx, CreateChannelInput{
		StudioID:      studioID,
		Kind:          KindTelegram,
		BSP:           "telegram",
		ExternalID:    botExternalID,
		DisplayHandle: "@test_ai_default_bot",
		AccessToken:   string(creds),
	})
	if err != nil {
		t.Fatalf("create test channel: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (SELECT lead_id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id = $1`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, channel.ID)
	})

	testChatID := int64(900000001)
	msg := func(text string, msgID int64) channels.TelegramUpdate {
		return channels.TelegramUpdate{
			UpdateID: msgID,
			Message: &channels.TelegramMessage{
				MessageID: msgID,
				Date:      1234567890,
				Text:      text,
				Chat:      channels.TelegramChat{ID: testChatID, Type: "private"},
				From:      &channels.TelegramUser{ID: testChatID, FirstName: "Ada"},
			},
		}
	}

	// --- first message creates the conversation with AI already on ---
	if err := msgSvc.HandleInboundTelegramMessage(ctx, botExternalID, msg("first message", 1)); err != nil {
		t.Fatalf("HandleInboundTelegramMessage (first): %v", err)
	}

	var convID uuid.UUID
	var aiEnabled bool
	if err := pool.QueryRow(ctx, `
		SELECT id, ai_enabled FROM conversations WHERE channel_account_id = $1
	`, channel.ID).Scan(&convID, &aiEnabled); err != nil {
		t.Fatalf("query conversation: %v", err)
	}
	if !aiEnabled {
		t.Fatal("ai_enabled = false on a newly created telegram bot conversation, want true (default-on)")
	}

	// --- staff turns AI off for this conversation ---
	if err := msgRepo.SetConversationAIEnabled(ctx, studioID, convID, false); err != nil {
		t.Fatalf("SetConversationAIEnabled(false): %v", err)
	}

	// --- a later inbound message must NOT silently re-enable it ---
	if err := msgSvc.HandleInboundTelegramMessage(ctx, botExternalID, msg("second message", 2)); err != nil {
		t.Fatalf("HandleInboundTelegramMessage (second): %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT ai_enabled FROM conversations WHERE id = $1`, convID).Scan(&aiEnabled); err != nil {
		t.Fatalf("query conversation after second message: %v", err)
	}
	if aiEnabled {
		t.Fatal("ai_enabled was forced back to true by a later message — staff's manual off must stick")
	}
}

// TestTelegramBot_NeverAutoCreatesLead verifies bot conversations stay
// Inbox-only — no lead gets auto-created even when the studio has a valid
// active campaign (which would normally be enough for other channels to
// auto-create one). Pipeline/Leads should only ever be populated by the
// QR channel (HandleInboundTGWeb), which keeps its own auto-create logic.
func TestTelegramBot_NeverAutoCreatesLead(t *testing.T) {
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

	// Needs a studio that actually HAS an active campaign — otherwise this
	// test can't distinguish "no lead because no campaign" (already covered
	// by TestTelegramWebhook_Integration) from "no lead because bot
	// deliberately never creates one" (what this test is actually for).
	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT c.studio_id FROM campaigns c WHERE c.active = true LIMIT 1
	`).Scan(&studioID); err != nil {
		t.Skip("Skipping test; no studio with an active campaign found in DB")
	}

	keyB64 := os.Getenv("TOKEN_ENCRYPTION_KEY")
	cipher, err := secrets.New(keyB64)
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}

	msgRepo := NewRepo(pool, cipher)
	msgBus := NewInProcBus()
	msgSvc := NewService(msgRepo, msgBus, "", "")

	botExternalID := fmt.Sprintf("test-no-lead-bot-%s", uuid.NewString()[:8])
	creds, _ := json.Marshal(channels.TelegramCredentials{BotToken: "fake-token", WebhookSecret: "shh"})
	channel, err := msgRepo.CreateChannel(ctx, CreateChannelInput{
		StudioID:      studioID,
		Kind:          KindTelegram,
		BSP:           "telegram",
		ExternalID:    botExternalID,
		DisplayHandle: "@test_no_lead_bot",
		AccessToken:   string(creds),
	})
	if err != nil {
		t.Fatalf("create test channel: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM leads WHERE id IN (SELECT lead_id FROM conversations WHERE channel_account_id = $1)`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE channel_account_id = $1`, channel.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM channel_accounts WHERE id = $1`, channel.ID)
	})

	testChatID := int64(900000099)
	upd := channels.TelegramUpdate{
		UpdateID: 1,
		Message: &channels.TelegramMessage{
			MessageID: 1,
			Date:      1234567890,
			Text:      "I want to try a class",
			Chat:      channels.TelegramChat{ID: testChatID, Type: "private"},
			From:      &channels.TelegramUser{ID: testChatID, FirstName: "Ada"},
		},
	}
	if err := msgSvc.HandleInboundTelegramMessage(ctx, botExternalID, upd); err != nil {
		t.Fatalf("HandleInboundTelegramMessage: %v", err)
	}

	var convID uuid.UUID
	var leadID *uuid.UUID
	var msgCount int
	if err := pool.QueryRow(ctx, `
		SELECT c.id, c.lead_id, (SELECT count(*) FROM messages m WHERE m.conversation_id = c.id)
		FROM conversations c WHERE c.channel_account_id = $1
	`, channel.ID).Scan(&convID, &leadID, &msgCount); err != nil {
		t.Fatalf("query conversation: %v", err)
	}

	// The conversation and message must still exist (Inbox stays populated)...
	if msgCount != 1 {
		t.Fatalf("message count = %d, want 1 — the message itself must still be recorded", msgCount)
	}
	// ...but no lead should ever be created for it.
	if leadID != nil {
		t.Fatalf("conversation.lead_id = %v, want nil — bot conversations must never auto-create a lead", *leadID)
	}
	var leadCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM leads WHERE source = 'telegram'`).Scan(&leadCount); err != nil {
		t.Fatalf("count telegram-sourced leads: %v", err)
	}
	if leadCount != 0 {
		t.Fatalf("found %d lead(s) with source='telegram' — bot must never write to leads at all", leadCount)
	}
}
