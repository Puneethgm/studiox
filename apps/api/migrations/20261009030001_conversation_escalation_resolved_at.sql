-- +goose Up
-- Stamped on resolve, left set afterward — lets a Dashboard KPI count
-- "currently resolved" separately from "never escalated".
ALTER TABLE conversations
    ADD COLUMN escalation_resolved_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN escalation_resolved_at;
