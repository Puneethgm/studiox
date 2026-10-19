-- +goose Up
-- +goose StatementBegin
-- studio_deletion_requests tracks the confirmation gate of the account
-- deletion flow (init.md §4.5): a studio admin requests deletion, a
-- single-use token is emailed, and the row moves pending -> confirmed ->
-- completed as the flow proceeds. No FK to studios (same reasoning as
-- audit_logs): a completed request must stay queryable for audit purposes
-- after the studio row it refers to has been permanently deleted.
CREATE TABLE studio_deletion_requests (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id            UUID NOT NULL,
    requested_by_user_id UUID NOT NULL,
    confirm_token_hash   TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','confirmed','completed','expired')),
    requested_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at         TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,
    expires_at           TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_studio_deletion_requests_studio ON studio_deletion_requests(studio_id);
-- Token hashes are looked up directly from the emailed confirm link, and
-- must be unique so a lookup is always unambiguous.
CREATE UNIQUE INDEX idx_studio_deletion_requests_token_hash ON studio_deletion_requests(confirm_token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS studio_deletion_requests;
-- +goose StatementEnd
