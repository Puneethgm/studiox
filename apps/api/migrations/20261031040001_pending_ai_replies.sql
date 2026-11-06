-- +goose Up
-- +goose StatementBegin
-- pending_ai_replies is ai-reply-flow-llm-design task 5's durable
-- debounce (doc §4.1) — replaces the in-memory time.AfterFunc debounce
-- in ai_worker.go's debounceInboundMessage, which drops a pending reply
-- if the process restarts mid-window. One row per conversation with a
-- pending reply; a new inbound message during the window updates
-- fire_at (the same "reset the timer" behavior the in-memory version
-- had), and a staff message deletes the row outright (task 5.3).
CREATE TABLE pending_ai_replies (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    message_id      UUID NOT NULL,
    fire_at         TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_pending_ai_replies_fire_at ON pending_ai_replies(fire_at) WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pending_ai_replies;
-- +goose StatementEnd
