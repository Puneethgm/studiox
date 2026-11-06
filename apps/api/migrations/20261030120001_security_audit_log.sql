-- +goose Up
-- +goose StatementBegin
-- security_audit_log is platform-security-hardening's event log — distinct
-- from audit_logs (owned by add-studio-rbac-and-audit-trail, which records
-- row-provenance for compliance flows). This table is append-only, never
-- edited/deleted by the app, and retained 12 months (not audit_logs' 90
-- days) — no FK with an ON DELETE behavior for the same reason audit_logs
-- has none: the trail must survive deletion of the studio/user it
-- describes.
CREATE TABLE security_audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID,
    actor_role    TEXT,
    studio_id     UUID,
    action        TEXT NOT NULL,
    target_type   TEXT,
    target_id     TEXT,
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip            TEXT,
    user_agent    TEXT,
    country       TEXT,
    session_id    UUID,
    outcome       TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_security_audit_log_studio_created ON security_audit_log(studio_id, created_at);
CREATE INDEX idx_security_audit_log_actor_created ON security_audit_log(actor_user_id, created_at);
CREATE INDEX idx_security_audit_log_action_created ON security_audit_log(action, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS security_audit_log;
-- +goose StatementEnd
