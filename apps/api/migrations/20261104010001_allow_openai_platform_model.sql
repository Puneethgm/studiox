-- +goose Up
-- platform_ai_models never allowed 'openai', so adding any model under the
-- OpenAI card in Settings always failed with a generic 500 (the API key
-- test passes, but the insert then violates this CHECK constraint). The
-- OpenAI provider is otherwise fully wired up (chatReplyProviders in
-- ai_models_http.go, ai_feature_routes.providers) — this constraint was
-- just never updated when OpenAI was added as a fourth chat-reply provider.
ALTER TABLE platform_ai_models DROP CONSTRAINT platform_ai_models_provider_check;
ALTER TABLE platform_ai_models ADD CONSTRAINT platform_ai_models_provider_check
    CHECK (provider IN ('groq','gemini','claude','openai'));

-- +goose Down
DELETE FROM platform_ai_models WHERE provider = 'openai';
ALTER TABLE platform_ai_models DROP CONSTRAINT platform_ai_models_provider_check;
ALTER TABLE platform_ai_models ADD CONSTRAINT platform_ai_models_provider_check
    CHECK (provider IN ('groq','gemini','claude'));
