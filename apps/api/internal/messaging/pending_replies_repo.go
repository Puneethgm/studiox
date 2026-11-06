package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PendingAIReply is one durable debounce row (ai-reply-flow-llm-design
// task 5.1) — see pending_ai_replies' migration comment.
type PendingAIReply struct {
	ConversationID uuid.UUID
	StudioID       uuid.UUID
	MessageID      uuid.UUID
	FireAt         time.Time
}

// UpsertPendingAIReply records (or resets) a pending reply for
// conversationID, firing at fireAt — the durable equivalent of the old
// in-memory timer's "a new message during the window resets it" behavior:
// a second call for the same conversation overwrites messageID/fireAt
// rather than creating a second row.
func (r *Repo) UpsertPendingAIReply(ctx context.Context, conversationID, studioID, messageID uuid.UUID, fireAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pending_ai_replies (conversation_id, studio_id, message_id, fire_at, status)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (conversation_id) DO UPDATE
		SET studio_id = EXCLUDED.studio_id, message_id = EXCLUDED.message_id,
		    fire_at = EXCLUDED.fire_at, status = 'pending'
	`, conversationID, studioID, messageID, fireAt)
	if err != nil {
		return fmt.Errorf("upsert pending ai reply: %w", err)
	}
	return nil
}

// ClaimDuePendingAIReplies atomically claims (marks 'processing', so a
// concurrent poll tick never double-fires the same row) every pending
// row whose fire_at has elapsed, and returns them for the caller to
// actually process.
func (r *Repo) ClaimDuePendingAIReplies(ctx context.Context, now time.Time) ([]PendingAIReply, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE pending_ai_replies
		SET status = 'processing'
		WHERE conversation_id IN (
			SELECT conversation_id FROM pending_ai_replies
			WHERE status = 'pending' AND fire_at <= $1
		)
		RETURNING conversation_id, studio_id, message_id, fire_at
	`, now)
	if err != nil {
		return nil, fmt.Errorf("claim due pending ai replies: %w", err)
	}
	defer rows.Close()

	var out []PendingAIReply
	for rows.Next() {
		var p PendingAIReply
		if err := rows.Scan(&p.ConversationID, &p.StudioID, &p.MessageID, &p.FireAt); err != nil {
			return nil, fmt.Errorf("scan pending ai reply: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePendingAIReply removes conversationID's pending row — called
// both after successfully claiming+processing one (cleanup) and when a
// staff message cancels a still-pending one (task 5.3).
func (r *Repo) DeletePendingAIReply(ctx context.Context, conversationID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM pending_ai_replies WHERE conversation_id = $1`, conversationID)
	if err != nil {
		return fmt.Errorf("delete pending ai reply: %w", err)
	}
	return nil
}
