package studios

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestStudiosRepo_ReplyTone covers the reply_tone column added by
// migrations/20261102010001_reply_tone.sql: the default ("professional",
// matching the previously hardcoded base-prompt wording), a valid
// round-trip for each allowed value, and rejection of an invalid value.
func TestStudiosRepo_ReplyTone(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}

	original, err := repo.GetByID(ctx, studioID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.SetReplyTone(context.Background(), studioID, original.ReplyTone)
	})

	if original.ReplyTone == "" {
		t.Error("ReplyTone should never be empty — the DB column has a NOT NULL DEFAULT 'professional'")
	}

	for _, tone := range []string{"professional", "casual", "friendly"} {
		if err := repo.SetReplyTone(ctx, studioID, tone); err != nil {
			t.Fatalf("SetReplyTone(%q): %v", tone, err)
		}
		got, err := repo.GetByID(ctx, studioID)
		if err != nil {
			t.Fatalf("GetByID after SetReplyTone(%q): %v", tone, err)
		}
		if got.ReplyTone != tone {
			t.Errorf("after SetReplyTone(%q), GetByID.ReplyTone = %q, want %q", tone, got.ReplyTone, tone)
		}
	}

	if err := repo.SetReplyTone(ctx, studioID, "sarcastic"); err != ErrValidation {
		t.Errorf("SetReplyTone(invalid) = %v, want ErrValidation", err)
	}

	if err := repo.SetReplyTone(ctx, uuid.New(), "professional"); err != ErrNotFound {
		t.Errorf("SetReplyTone(nonexistent studio) = %v, want ErrNotFound", err)
	}
}
