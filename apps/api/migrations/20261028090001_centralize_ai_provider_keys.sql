-- +goose Up
-- Centralizes Groq/Gemini/Claude API keys and model selection under
-- superadmin control instead of each studio bringing its own key/model
-- list — see BIFROST_MIGRATION_PLAN.md and studios.Repo.
-- GetPlatformAIProviderKey. Mistral (OCR only, not a chat-reply provider)
-- stays per-studio and is untouched here.
ALTER TABLE studios DROP COLUMN IF EXISTS groq_api_key;
ALTER TABLE studios DROP COLUMN IF EXISTS gemini_api_key;
ALTER TABLE studios DROP COLUMN IF EXISTS claude_api_key;

-- studio_ai_models only ever held groq/gemini/claude rows (its own CHECK
-- constraint never allowed mistral), so it has no reason to exist once
-- those three providers move to a platform-wide model list.
DROP TABLE IF EXISTS studio_ai_models;

CREATE TABLE platform_ai_models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider        TEXT NOT NULL CHECK (provider IN ('groq','gemini','claude')),
    model_name      TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    sort_order      INT NOT NULL DEFAULT 0,
    last_tested_at  TIMESTAMPTZ,
    last_test_ok    BOOLEAN,
    last_test_error TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, model_name)
);

-- +goose Down
ALTER TABLE studios ADD COLUMN IF NOT EXISTS groq_api_key TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN IF NOT EXISTS gemini_api_key TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN IF NOT EXISTS claude_api_key TEXT NOT NULL DEFAULT '';

DROP TABLE IF EXISTS platform_ai_models;

CREATE TABLE studio_ai_models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL CHECK (provider IN ('groq','gemini','claude')),
    model_name      TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    sort_order      INT NOT NULL DEFAULT 0,
    last_tested_at  TIMESTAMPTZ,
    last_test_ok    BOOLEAN,
    last_test_error TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (studio_id, provider, model_name)
);

CREATE INDEX idx_studio_ai_models_studio ON studio_ai_models(studio_id);
