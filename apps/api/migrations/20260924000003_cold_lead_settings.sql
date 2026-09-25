-- +goose Up
-- Per-studio configurable thresholds for the Cold Leads scanner (see
-- internal/messaging/cold_scanner.go). Defaults match the values that were
-- previously hardcoded in ListColdLeads.
ALTER TABLE studios ADD COLUMN cold_never_replied_days INT NOT NULL DEFAULT 1;
ALTER TABLE studios ADD COLUMN cold_stalled_days INT NOT NULL DEFAULT 7;

-- Persisted cold status, written by the periodic scanner instead of being
-- computed live on every Pipeline load. cold_reason is 'never_replied',
-- 'stalled', or NULL (not cold).
ALTER TABLE conversations ADD COLUMN cold_reason TEXT;
ALTER TABLE conversations ADD COLUMN cold_detected_at TIMESTAMPTZ;

CREATE INDEX idx_conversations_cold ON conversations (studio_id) WHERE cold_reason IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_conversations_cold;
ALTER TABLE conversations DROP COLUMN cold_detected_at;
ALTER TABLE conversations DROP COLUMN cold_reason;
ALTER TABLE studios DROP COLUMN cold_stalled_days;
ALTER TABLE studios DROP COLUMN cold_never_replied_days;
