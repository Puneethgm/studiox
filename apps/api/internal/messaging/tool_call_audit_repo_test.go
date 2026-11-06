package messaging

import (
	"context"
	"testing"
)

// TestRecordToolCallAudit_RealRow covers task 7.4: a real tool call
// (success and failure both) produces a queryable audit row.
func TestRecordToolCallAudit_RealRow(t *testing.T) {
	repo, pool, studioID, conversationID := setupPendingRepliesTest(t)
	ctx := context.Background()

	errMsg := "class full"
	if err := repo.RecordToolCallAudit(ctx, conversationID, studioID, "create_booking",
		[]byte(`{"user_id":"u1","event_id":"evt1"}`), nil, &errMsg); err != nil {
		t.Fatalf("RecordToolCallAudit (failure case): %v", err)
	}
	if err := repo.RecordToolCallAudit(ctx, conversationID, studioID, "check_availability",
		[]byte(`{"from":0,"to":1000}`), []byte(`[{"_id":"evt1"}]`), nil); err != nil {
		t.Fatalf("RecordToolCallAudit (success case): %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tool_call_audit WHERE conversation_id = $1`, conversationID)
	})

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tool_call_audit WHERE conversation_id = $1`, conversationID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 audit rows, got %d", count)
	}

	var tool string
	var errCol *string
	if err := pool.QueryRow(ctx, `SELECT tool, error FROM tool_call_audit WHERE conversation_id = $1 AND tool = 'create_booking'`, conversationID).
		Scan(&tool, &errCol); err != nil {
		t.Fatalf("query failure row: %v", err)
	}
	if errCol == nil || *errCol != "class full" {
		t.Errorf("error column = %v, want %q", errCol, "class full")
	}
}
