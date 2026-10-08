package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// Real bug report: in a live WhatsApp conversation that organically grew
// past 15 messages, the AI's own reply said "Apologies for the mix-up—I
// don't have access to your previous chat history here... Could you quickly
// remind me of your main fitness goals" — re-asking something the customer
// had already told it, because handleMessage's ListMessages(...,
// aiHistoryWindow) call only ever loads the most recent aiHistoryWindow
// messages, and summarization (ai_context_summary, which exists precisely
// to cover what falls out of that window) previously only ever ran once,
// right after a wa-web chat-history backfill import — never during an
// ordinary live conversation as it grows. ensureRollingSummary/
// shouldRefreshRollingSummary close that gap.
func TestShouldRefreshRollingSummary(t *testing.T) {
	cases := []struct {
		total int
		want  bool
	}{
		{0, false},
		{1, false},
		{aiHistoryWindow, false},     // exactly fills the window — nothing has fallen out yet
		{aiHistoryWindow + 1, true},  // the real bug case: first message to fall out of the window
		{aiHistoryWindow + 2, false}, // not yet another multiple of rollingSummaryRefreshEvery
		{aiHistoryWindow + rollingSummaryRefreshEvery, true},
		{aiHistoryWindow + rollingSummaryRefreshEvery + 1, false},
		{aiHistoryWindow + 2*rollingSummaryRefreshEvery, true},
		{500, 500%rollingSummaryRefreshEvery == 0}, // ListMessages' own cap — must still evaluate sanely, not panic
	}
	for _, c := range cases {
		if got := shouldRefreshRollingSummary(c.total); got != c.want {
			t.Errorf("shouldRefreshRollingSummary(%d) = %v, want %v", c.total, got, c.want)
		}
	}
}

// Confirms ensureRollingSummary actually reaches into the DB and slices off
// exactly the "older" portion (everything except the most recent
// aiHistoryWindow messages) to summarize — not the whole conversation, and
// not just the recent window (which would be redundant with what the prompt
// already includes directly).
func TestEnsureRollingSummary_SummarizesOnlyWhatFellOutOfTheWindow(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	msgRepo := NewRepo(pool, cipher)

	var studioID uuid.UUID
	var kindStr string
	if err := pool.QueryRow(ctx, `
		SELECT c.studio_id, c.kind
		FROM channel_accounts c
		JOIN campaigns camp ON camp.studio_id = c.studio_id
		WHERE c.status = 'active'
		LIMIT 1
	`).Scan(&studioID, &kindStr); err != nil {
		t.Skip("skipping test; no studio with both an active channel and a campaign found in DB")
	}

	conv, err := NewService(msgRepo, NewInProcBus(), "", "", nil, nil, "").CreateConversation(ctx, studioID, CreateConversationInput{
		ChannelKind:  ChannelKind(kindStr),
		ContactValue: "6591234567",
		DisplayName:  "Rolling Summary Test",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	// Seed aiHistoryWindow+rollingSummaryRefreshEvery messages (a count that
	// actually satisfies shouldRefreshRollingSummary's trigger condition):
	// the first rollingSummaryRefreshEvery ("older") must end up in the
	// summary input; the rest ("recent window") must not.
	olderCount := rollingSummaryRefreshEvery
	totalToSeed := aiHistoryWindow + olderCount
	for i := 0; i < totalToSeed; i++ {
		body := "older message"
		if i >= olderCount {
			body = "recent window message"
		}
		if _, err := msgRepo.pool.Exec(ctx, `
			INSERT INTO messages (conversation_id, studio_id, direction, source_kind, body, status, sent_at)
			VALUES ($1,$2,'inbound','studio_user',$3,'sent', now() - make_interval(secs => $4))
		`, conv.ID, studioID, body, totalToSeed-i); err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE conversation_id = $1`, conv.ID)
	})

	full, err := msgRepo.ListMessages(ctx, studioID, conv.ID, 500)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(full) != totalToSeed {
		t.Fatalf("seeded %d messages, got %d back", totalToSeed, len(full))
	}
	if !shouldRefreshRollingSummary(len(full)) {
		t.Fatalf("shouldRefreshRollingSummary(%d) = false, want true", len(full))
	}

	older := full[:len(full)-aiHistoryWindow]
	if len(older) != olderCount {
		t.Fatalf("older slice = %d messages, want %d", len(older), olderCount)
	}
	for _, m := range older {
		if m.Body != "older message" {
			t.Errorf("older slice contained a recent-window message: %q", m.Body)
		}
	}
}
