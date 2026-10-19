-- +goose Up
-- +goose StatementBegin
-- account_archives is the encrypted, post-deletion data export a studio
-- owner can recover within a 7-day window (init.md §4.5's "recovery link").
-- No FK to studios: it must outlive the studio row it was built from — that
-- studio no longer exists by the time this row is written.
CREATE TABLE account_archives (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id           UUID NOT NULL,
    s3_key              TEXT NOT NULL,
    -- Envelope encryption: the archive itself is encrypted with a random
    -- per-archive data key, and this column holds that data key after it has
    -- been encrypted ("wrapped") with the platform's master cipher — see
    -- internal/studios/archive.go's EncryptArchive/DecryptArchive.
    wrapped_data_key    TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL,
    recovery_token_hash TEXT NOT NULL,
    -- Snapshot of the studio's contact email at deletion time, used to
    -- re-verify the requester on recovery without requiring a (possibly
    -- stale) password for an account that no longer exists.
    recovery_email      TEXT NOT NULL,
    recovery_expires_at TIMESTAMPTZ NOT NULL,
    downloaded_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_account_archives_recovery_token_hash ON account_archives(recovery_token_hash);
CREATE INDEX idx_account_archives_studio ON account_archives(studio_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS account_archives;
-- +goose StatementEnd
