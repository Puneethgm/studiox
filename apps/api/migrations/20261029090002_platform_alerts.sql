-- +goose Up
-- Records platform-level AI alert conditions (all providers failed on a
-- customer reply, a provider key failing, provider failure-rate spikes,
-- daily spend over budget) so they're visible to superadmins instead of
-- only appearing as a log line. See
-- openspec/changes/llm-gateway-and-key-management.
CREATE TABLE IF NOT EXISTS platform_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    studio_id UUID REFERENCES studios(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_platform_alerts_kind_created_at ON platform_alerts (kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_platform_alerts_unresolved ON platform_alerts (created_at DESC) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS platform_alerts;
