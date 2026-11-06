-- +goose Up
-- Envelope-encrypted platform provider key store, replacing the single
-- static-key AES storage of the key under platform_settings
-- (groq_api_key/gemini_api_key/claude_api_key/openai_api_key). Each row's
-- key_ciphertext is encrypted with its own data_key; data_key_ciphertext is
-- that data key encrypted with the app's master key (TOKEN_ENCRYPTION_KEY,
-- never stored here). See
-- openspec/changes/llm-gateway-and-key-management/design.md Decision 4.
CREATE TABLE IF NOT EXISTS platform_ai_keys (
    provider TEXT PRIMARY KEY,
    key_ciphertext TEXT NOT NULL,
    data_key_ciphertext TEXT NOT NULL,
    last_four TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS platform_ai_key_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    action TEXT NOT NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    detail JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_platform_ai_key_audit_provider ON platform_ai_key_audit (provider, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS platform_ai_key_audit;
DROP TABLE IF EXISTS platform_ai_keys;
