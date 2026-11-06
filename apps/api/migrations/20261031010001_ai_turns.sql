-- +goose Up
-- +goose StatementBegin
-- ai_turns is ai-reply-flow-llm-design's per-reply measurement record
-- (doc §8.9) — one row per generated AI reply (sent, guard-failed, or
-- handed-off), carrying enough to reproduce/grade it offline against the
-- eval case set and to compute the doc's success-criteria metrics in
-- production (calls/reply, silent-message rate, wrong-claim rate,
-- latency, cost).
CREATE TABLE ai_turns (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    studio_id         UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    pipeline_version  TEXT NOT NULL,
    structured_output JSONB,
    guard_outcome     TEXT NOT NULL DEFAULT 'passed',
    tool_calls        JSONB,
    latency_ms        INTEGER,
    cost_usd          NUMERIC(10, 6),
    outcome           TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_turns_conversation_id ON ai_turns(conversation_id);
CREATE INDEX idx_ai_turns_studio_id_created_at ON ai_turns(studio_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ai_turns;
-- +goose StatementEnd
