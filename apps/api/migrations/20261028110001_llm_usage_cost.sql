-- +goose Up
-- Bifrost prices every call against its own provider rate catalog and
-- returns the result on the response (BifrostLLMUsage.Cost.TotalCost) —
-- this just stops us from throwing that number away.
ALTER TABLE llm_usage_logs ADD COLUMN IF NOT EXISTS cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE llm_usage_logs DROP COLUMN IF EXISTS cost_usd;
