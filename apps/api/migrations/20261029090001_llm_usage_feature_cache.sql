-- +goose Up
-- Attributes each LLM call to the feature that made it (customer_reply,
-- summary, style_profile, intent_classify, query_expand, rerank) and marks
-- whether it was served from the answer cache instead of a live provider
-- call, so cost/volume can be broken down per feature instead of only per
-- provider/model. See openspec/changes/llm-gateway-and-key-management.
ALTER TABLE llm_usage_logs ADD COLUMN IF NOT EXISTS feature TEXT;
ALTER TABLE llm_usage_logs ADD COLUMN IF NOT EXISTS cache_hit BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_llm_usage_logs_feature ON llm_usage_logs (feature);

-- +goose Down
DROP INDEX IF EXISTS idx_llm_usage_logs_feature;
ALTER TABLE llm_usage_logs DROP COLUMN IF EXISTS cache_hit;
ALTER TABLE llm_usage_logs DROP COLUMN IF EXISTS feature;
