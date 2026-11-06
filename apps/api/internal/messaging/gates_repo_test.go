package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestTodayAISpendUSDCents_ReflectsRealUsageLogs covers task 6.3's data
// source directly: a real llm_usage_logs row for today is reflected in
// the sum, and a row from before today is not.
func TestTodayAISpendUSDCents_ReflectsRealUsageLogs(t *testing.T) {
	repo, pool, studioID, _ := setupPendingRepliesTest(t)
	ctx := context.Background()

	before, err := repo.TodayAISpendUSDCents(ctx, studioID)
	if err != nil {
		t.Fatalf("TodayAISpendUSDCents (before): %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO llm_usage_logs (studio_id, provider, model, cost_usd, created_at)
		VALUES ($1, 'test', 'test-model', 0.05, now())
	`, studioID); err != nil {
		t.Fatalf("insert usage log: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM llm_usage_logs WHERE provider = 'test' AND studio_id = $1`, studioID)
	})

	after, err := repo.TodayAISpendUSDCents(ctx, studioID)
	if err != nil {
		t.Fatalf("TodayAISpendUSDCents (after): %v", err)
	}
	if after != before+5 {
		t.Errorf("TodayAISpendUSDCents = %d, want %d (before=%d + 5 cents for the $0.05 row)", after, before+5, before)
	}
}

// TestTodayAIReplyCountForConversation_CountsOnlyAIOutboundToday covers
// task 6.4's data source: counts only source_kind='ai' outbound messages
// sent today for the given conversation.
func TestTodayAIReplyCountForConversation_CountsOnlyAIOutboundToday(t *testing.T) {
	repo, pool, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	before, err := repo.TodayAIReplyCountForConversation(ctx, conversationID)
	if err != nil {
		t.Fatalf("TodayAIReplyCountForConversation (before): %v", err)
	}

	insertedID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO messages (id, conversation_id, studio_id, direction, source_kind, body, sent_at)
		VALUES ($1, $2, $3, 'outbound', 'ai', 'test reply', now())
	`, insertedID, conversationID, studioID); err != nil {
		t.Fatalf("insert test message: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE id = $1`, insertedID)
	})

	after, err := repo.TodayAIReplyCountForConversation(ctx, conversationID)
	if err != nil {
		t.Fatalf("TodayAIReplyCountForConversation (after): %v", err)
	}
	if after != before+1 {
		t.Errorf("TodayAIReplyCountForConversation = %d, want %d", after, before+1)
	}
}

// TestCheckReplyGates_DailyReplyCapBlocks is a focused live test of
// checkReplyGates itself (not just the repo helper) proving the 6.4 gate
// actually fires: 30 AI-sourced messages inserted for a conversation
// today makes checkReplyGates return "daily_reply_cap_exceeded".
func TestCheckReplyGates_DailyReplyCapBlocks(t *testing.T) {
	repo, pool, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	var insertedIDs []uuid.UUID
	for i := 0; i < dailyReplyCap; i++ {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (id, conversation_id, studio_id, direction, source_kind, body, sent_at)
			VALUES ($1, $2, $3, 'outbound', 'ai', 'test reply', now())
		`, id, conversationID, studioID); err != nil {
			t.Fatalf("insert test message %d: %v", i, err)
		}
		insertedIDs = append(insertedIDs, id)
	}
	t.Cleanup(func() {
		for _, id := range insertedIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE id = $1`, id)
		}
	})

	w := &AIWorker{msgRepo: repo}
	conv := &Conversation{ID: conversationID, StudioID: studioID}
	reason := w.checkReplyGates(ctx, studioID, conv, nil, []Message{{SentAt: time.Now()}})
	if reason != "daily_reply_cap_exceeded" {
		t.Errorf("checkReplyGates() = %q, want %q after %d AI replies today", reason, "daily_reply_cap_exceeded", dailyReplyCap)
	}
}

// TestCheckReplyGates_AIActiveHours covers the new, dedicated "AI Active
// Hours" gate end to end through checkReplyGates: disabled (the
// migration's default) never blocks regardless of time, and enabled with
// a window that excludes right now blocks with "outside_ai_active_hours".
func TestCheckReplyGates_AIActiveHours(t *testing.T) {
	repo, _, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()
	w := &AIWorker{msgRepo: repo}
	conv := &Conversation{ID: conversationID, StudioID: studioID}
	history := []Message{{SentAt: time.Now().Add(-time.Hour)}}

	t.Run("disabled by default never blocks", func(t *testing.T) {
		reason := w.checkReplyGates(ctx, studioID, conv, nil, history)
		if reason == "outside_ai_active_hours" {
			t.Error("checkReplyGates() blocked on outside_ai_active_hours while the setting is disabled by default")
		}
	})

	t.Run("enabled with a window excluding now blocks", func(t *testing.T) {
		now := time.Now().UTC()
		start := now.Add(2 * time.Hour)
		end := now.Add(3 * time.Hour)
		if err := repo.SetAIActiveHours(ctx, studioID, true, start.Format("15:04"), end.Format("15:04"), "UTC"); err != nil {
			t.Fatalf("SetAIActiveHours: %v", err)
		}
		t.Cleanup(func() {
			_ = repo.SetAIActiveHours(context.Background(), studioID, false, "00:00", "23:59", "UTC")
		})

		reason := w.checkReplyGates(ctx, studioID, conv, nil, history)
		if reason != "outside_ai_active_hours" {
			t.Errorf("checkReplyGates() = %q, want %q for a window 2-3 hours from now", reason, "outside_ai_active_hours")
		}
	})
}
