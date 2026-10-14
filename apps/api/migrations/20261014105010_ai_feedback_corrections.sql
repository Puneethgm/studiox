-- +goose Up
-- Staff feedback on a specific AI reply ("this was wrong, it should have
-- said X instead"), given from the Knowledge Base Test Chat drawer. Folded
-- into buildPrompt (see AIWorker.buildPrompt's STAFF CORRECTIONS section)
-- for every future reply in this studio — both test chat and real live
-- replies share buildPrompt, so a correction given in testing also improves
-- production answers, not just the test session it was given in.
CREATE TABLE ai_feedback_corrections (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id     uuid        NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    question      text        NOT NULL,
    original_reply text       NOT NULL DEFAULT '',
    correction    text        NOT NULL,
    source        text        NOT NULL DEFAULT 'test_chat' CHECK (source IN ('test_chat', 'live')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    created_by    uuid        REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_ai_feedback_corrections_studio ON ai_feedback_corrections(studio_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_feedback_corrections;
