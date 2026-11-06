package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func setupPendingRepliesTest(t *testing.T) (*Repo, *pgxpool.Pool, uuid.UUID, uuid.UUID) {
	t.Helper()
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

	var studioID, conversationID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT studio_id, id FROM conversations LIMIT 1`).Scan(&studioID, &conversationID); err != nil {
		t.Skip("skipping test; no conversation found in DB")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pending_ai_replies WHERE conversation_id = $1`, conversationID)
	})

	return &Repo{pool: pool}, pool, studioID, conversationID
}

// TestPendingAIReplies_FiresOnceFireAtElapses covers task 5.2's "durable"
// claim directly — this upserts a row and claims it WITHOUT ever starting
// an in-memory timer, which is itself the proof of durability: nothing
// in-process is required for the row to exist and eventually fire.
func TestPendingAIReplies_FiresOnceFireAtElapses(t *testing.T) {
	repo, _, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()
	messageID := uuid.New()

	// fire_at in the past — simulates "the window already elapsed by the
	// time a (possibly different) process's poll loop checks."
	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, messageID, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("UpsertPendingAIReply: %v", err)
	}

	due, err := repo.ClaimDuePendingAIReplies(ctx, time.Now())
	if err != nil {
		t.Fatalf("ClaimDuePendingAIReplies: %v", err)
	}
	found := false
	for _, p := range due {
		if p.ConversationID == conversationID {
			found = true
			if p.MessageID != messageID {
				t.Errorf("claimed MessageID = %v, want %v", p.MessageID, messageID)
			}
		}
	}
	if !found {
		t.Fatal("expected the due pending reply to be claimed")
	}

	// Claiming again must NOT return the same row a second time (no
	// double-fire) — it was marked 'processing', not left 'pending'.
	due2, err := repo.ClaimDuePendingAIReplies(ctx, time.Now())
	if err != nil {
		t.Fatalf("ClaimDuePendingAIReplies (second call): %v", err)
	}
	for _, p := range due2 {
		if p.ConversationID == conversationID {
			t.Error("the same pending reply was claimed twice — status='processing' did not prevent a double-fire")
		}
	}
}

// TestPendingAIReplies_NotYetDue_NotClaimed confirms a row whose fire_at
// is still in the future isn't claimed early.
func TestPendingAIReplies_NotYetDue_NotClaimed(t *testing.T) {
	repo, _, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, uuid.New(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("UpsertPendingAIReply: %v", err)
	}

	due, err := repo.ClaimDuePendingAIReplies(ctx, time.Now())
	if err != nil {
		t.Fatalf("ClaimDuePendingAIReplies: %v", err)
	}
	for _, p := range due {
		if p.ConversationID == conversationID {
			t.Error("a pending reply whose fire_at is in the future should not be claimed yet")
		}
	}
}

// TestPendingAIReplies_UpsertResetsFireAtAndMessageID covers the
// "new message during the window resets it" debounce behavior.
func TestPendingAIReplies_UpsertResetsFireAtAndMessageID(t *testing.T) {
	repo, pool, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()
	firstMsg := uuid.New()
	secondMsg := uuid.New()

	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, firstMsg, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("first UpsertPendingAIReply: %v", err)
	}
	newFireAt := time.Now().Add(2 * time.Hour)
	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, secondMsg, newFireAt); err != nil {
		t.Fatalf("second UpsertPendingAIReply: %v", err)
	}

	var gotMsgID uuid.UUID
	var gotFireAt time.Time
	if err := pool.QueryRow(ctx, `SELECT message_id, fire_at FROM pending_ai_replies WHERE conversation_id = $1`, conversationID).
		Scan(&gotMsgID, &gotFireAt); err != nil {
		t.Fatalf("query pending row: %v", err)
	}
	if gotMsgID != secondMsg {
		t.Errorf("message_id = %v, want the second (most recent) message %v — upsert should reset it, not keep the first", gotMsgID, secondMsg)
	}
	if gotFireAt.Sub(newFireAt).Abs() > time.Second {
		t.Errorf("fire_at = %v, want ~%v", gotFireAt, newFireAt)
	}

	// Still exactly one row, not two.
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pending_ai_replies WHERE conversation_id = $1`, conversationID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for the conversation after two upserts, got %d", count)
	}
}

// TestPendingAIReplies_Delete covers removal — both the "fired
// successfully, clean up" path and the "staff message cancels it" path
// (task 5.3) use the same delete.
func TestPendingAIReplies_Delete(t *testing.T) {
	repo, _, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, uuid.New(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("UpsertPendingAIReply: %v", err)
	}
	if err := repo.DeletePendingAIReply(ctx, conversationID); err != nil {
		t.Fatalf("DeletePendingAIReply: %v", err)
	}
	due, err := repo.ClaimDuePendingAIReplies(ctx, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatalf("ClaimDuePendingAIReplies: %v", err)
	}
	for _, p := range due {
		if p.ConversationID == conversationID {
			t.Error("deleted pending reply should never be claimed")
		}
	}
}

// TestCancelPendingReplyIfStaffSent_RealStaffMessage_Cancels covers task
// 5.3 live: a real, already-persisted staff-authored message (source_kind
// = studio_user) cancels a pending debounced reply for its conversation.
func TestCancelPendingReplyIfStaffSent_RealStaffMessage_Cancels(t *testing.T) {
	repo, pool, studioID, _ := setupPendingRepliesTest(t)
	ctx := context.Background()

	var conversationID, messageID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT conversation_id, id FROM messages WHERE source_kind = 'studio_user' LIMIT 1
	`).Scan(&conversationID, &messageID); err != nil {
		t.Skip("skipping test; no studio_user-authored message found in DB")
	}
	// Re-derive studioID for the conversation this message actually
	// belongs to (may differ from the arbitrary one setupPendingRepliesTest
	// picked for the fixture conversation it cleans up).
	if err := pool.QueryRow(ctx, `SELECT studio_id FROM conversations WHERE id = $1`, conversationID).Scan(&studioID); err != nil {
		t.Fatalf("look up studio for conversation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pending_ai_replies WHERE conversation_id = $1`, conversationID)
	})

	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, uuid.New(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("UpsertPendingAIReply: %v", err)
	}

	w := &AIWorker{msgRepo: repo}
	w.cancelPendingReplyIfStaffSent(ctx, studioID, conversationID, messageID)

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pending_ai_replies WHERE conversation_id = $1`, conversationID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Error("a real staff-authored message should have cancelled the pending reply")
	}
}

// TestCancelPendingReplyIfStaffSent_NonStaffMessage_DoesNotCancel is the
// control case: a non-staff message (or an unknown one) must not cancel a
// pending reply.
func TestCancelPendingReplyIfStaffSent_NonStaffMessage_DoesNotCancel(t *testing.T) {
	repo, _, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	if err := repo.UpsertPendingAIReply(ctx, conversationID, studioID, uuid.New(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("UpsertPendingAIReply: %v", err)
	}

	w := &AIWorker{msgRepo: repo}
	// A message ID that doesn't exist — GetMessageByID returns nil/err,
	// which must be treated as "not a staff message," not a cancellation.
	w.cancelPendingReplyIfStaffSent(ctx, studioID, conversationID, uuid.New())

	due, err := repo.ClaimDuePendingAIReplies(ctx, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatalf("ClaimDuePendingAIReplies: %v", err)
	}
	found := false
	for _, p := range due {
		if p.ConversationID == conversationID {
			found = true
		}
	}
	if !found {
		t.Error("an unknown/non-staff message should not have cancelled the pending reply")
	}
}
