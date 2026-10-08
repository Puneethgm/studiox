-- +goose Up
-- Questions the AI couldn't answer from the knowledge base and had to escalate to a
-- human (see messaging.AIWorker's low-KB-confidence handoff). Surfaced on the
-- Knowledge Base page as a "Needs Answers" tab so an admin can see exactly what
-- customers asked and were missed, then add the answer straight into the KB.
CREATE TABLE knowledge_gaps (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       uuid        NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    conversation_id uuid        REFERENCES conversations(id) ON DELETE SET NULL,
    lead_id         uuid        REFERENCES leads(id) ON DELETE SET NULL,
    question        text        NOT NULL,
    status          text        NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved','dismissed')),
    times_asked     int         NOT NULL DEFAULT 1,
    answer          text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    resolved_at     timestamptz,
    resolved_by     uuid        REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_knowledge_gaps_studio_status ON knowledge_gaps(studio_id, status, created_at DESC);
-- Repeat asks of (near-)the same question while still unanswered bump times_asked
-- instead of piling up duplicate rows — case-insensitive, trimmed match.
CREATE UNIQUE INDEX idx_knowledge_gaps_open_dedupe
    ON knowledge_gaps(studio_id, lower(btrim(question)))
    WHERE status = 'open';

-- +goose Down
DROP TABLE IF EXISTS knowledge_gaps;
