-- +goose Up
-- +goose StatementBegin
-- audit_logs records every step of compliance-sensitive flows (account
-- deletion request/confirm, archive creation, recovery attempts, etc — see
-- init.md §4.5). Deliberately has NO foreign key to studios or users: an
-- audit trail must survive the deletion of the studio (and even the user)
-- it describes, so any FK with an ON DELETE behavior would either block the
-- very deletion it's recording or silently erase the audit trail along with
-- it. studio_id/actor_user_id are plain nullable columns instead.
CREATE TABLE audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id     UUID,
    actor_user_id UUID,
    action        TEXT NOT NULL,
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_studio_created ON audit_logs(studio_id, created_at);
CREATE INDEX idx_audit_logs_action_created ON audit_logs(action, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
-- +goose StatementEnd
