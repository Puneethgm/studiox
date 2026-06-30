-- +goose Up
CREATE TABLE IF NOT EXISTS llm_usage_logs (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  studio_id    UUID        REFERENCES studios(id) ON DELETE SET NULL,
  provider     TEXT        NOT NULL DEFAULT '',
  model        TEXT        NOT NULL DEFAULT '',
  latency_ms   INTEGER     NOT NULL DEFAULT 0,
  success      BOOLEAN     NOT NULL DEFAULT true,
  error_msg    TEXT        NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS llm_usage_logs_studio_id_idx ON llm_usage_logs(studio_id);
CREATE INDEX IF NOT EXISTS llm_usage_logs_created_at_idx ON llm_usage_logs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS llm_usage_logs;
