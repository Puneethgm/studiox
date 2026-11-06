-- +goose Up
-- Per-feature LLM routing config: which providers to try, in what order,
-- and with what per-call settings — replacing the hardcoded Go provider
-- sequence (Groq→Gemini→Claude→OpenAI) duplicated across ai_worker.go and
-- style_worker.go. Which MODEL(S) within a provider to try stays governed
-- by the existing platform_ai_models/EnabledModels system (unchanged) —
-- this table controls provider order and per-feature call settings only.
-- See openspec/changes/llm-gateway-and-key-management/design.md.
CREATE TABLE IF NOT EXISTS ai_feature_routes (
    feature TEXT PRIMARY KEY,
    route JSONB NOT NULL,
    time_budget_ms INTEGER NOT NULL DEFAULT 60000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Seed with today's hardcoded behavior so cutting features over to the
-- gateway changes nothing observable until a superadmin deliberately edits
-- a route. See bifrostAttemptProvider's call order in ai_worker.go and
-- style_worker.go's llmWaterfall for the sequence/settings this mirrors.
INSERT INTO ai_feature_routes (feature, route, time_budget_ms) VALUES
    ('customer_reply', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7},
        "min_usable_length": 15
    }'::jsonb, 60000),
    ('summary', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7}
    }'::jsonb, 60000),
    ('style_profile', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7}
    }'::jsonb, 60000),
    ('test_chat', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7}
    }'::jsonb, 60000),
    ('template_gen', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7}
    }'::jsonb, 60000),
    ('enable_ai_followup', '{
        "providers": ["groq", "gemini", "claude", "openai"],
        "max_tokens": {"groq": 512, "claude": 512, "openai": 512},
        "temperature": {"groq": 0.7}
    }'::jsonb, 60000)
ON CONFLICT (feature) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS ai_feature_routes;
