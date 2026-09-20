-- +goose Up
ALTER TABLE studios ADD COLUMN IF NOT EXISTS claude_api_key TEXT NOT NULL DEFAULT '';

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

-- +goose Down
DROP TABLE studio_ai_models;
ALTER TABLE studios DROP COLUMN IF EXISTS claude_api_key;
