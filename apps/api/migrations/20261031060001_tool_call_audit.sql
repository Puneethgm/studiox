-- +goose Up
-- +goose StatementBegin
-- tool_call_audit is ai-reply-flow-llm-design task 7.4's audit record —
-- every tool call the reply pipeline actually executes (not just
-- requests), regardless of success/failure.
CREATE TABLE tool_call_audit (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    tool            TEXT NOT NULL,
    arguments       JSONB NOT NULL DEFAULT '{}'::jsonb,
    result          JSONB,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tool_call_audit_conversation ON tool_call_audit(conversation_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tool_call_audit;
-- +goose StatementEnd
