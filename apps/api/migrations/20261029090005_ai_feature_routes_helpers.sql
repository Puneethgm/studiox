-- +goose Up
-- Routes for the three "helper" AI features (studios.ClassifyIntent,
-- ExpandQuery, RerankChunks) — previously standalone functions calling
-- Gemini-then-Groq directly (bifrostCompleteWithFallback), not through any
-- shared waterfall. Seeded with that exact same Gemini→Groq order and a
-- short time budget (these run inline in the reply pipeline, parallel with
-- KB retrieval — see ai_worker.go's handleMessage — so they must not eat
-- into the overall reply time budget the way a full customer-reply
-- waterfall can). See
-- openspec/changes/llm-gateway-and-key-management, task 3.6.
-- Per-feature max_tokens/temperature match each function's prior hardcoded
-- values exactly (ClassifyIntent: 80, ExpandQuery: 120, RerankChunks: 50;
-- all three used temperature 0 for deterministic structured output).
INSERT INTO ai_feature_routes (feature, route, time_budget_ms) VALUES
    ('intent_classify', '{"providers": ["gemini", "groq"], "max_tokens": {"gemini": 80, "groq": 80}, "temperature": {"gemini": 0, "groq": 0}}'::jsonb, 15000),
    ('query_expand', '{"providers": ["gemini", "groq"], "max_tokens": {"gemini": 120, "groq": 120}, "temperature": {"gemini": 0, "groq": 0}}'::jsonb, 15000),
    ('rerank', '{"providers": ["gemini", "groq"], "max_tokens": {"gemini": 50, "groq": 50}, "temperature": {"gemini": 0, "groq": 0}}'::jsonb, 15000)
ON CONFLICT (feature) DO NOTHING;

-- +goose Down
DELETE FROM ai_feature_routes WHERE feature IN ('intent_classify', 'query_expand', 'rerank');
