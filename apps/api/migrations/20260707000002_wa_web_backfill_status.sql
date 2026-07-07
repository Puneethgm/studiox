-- +goose Up

ALTER TABLE channel_accounts
    ADD COLUMN backfill_status TEXT NOT NULL DEFAULT 'none'
        CHECK (backfill_status IN ('none','running','done','failed')),
    ADD COLUMN backfill_message_count INT NOT NULL DEFAULT 0,
    ADD COLUMN backfill_updated_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE channel_accounts
    DROP COLUMN IF EXISTS backfill_status,
    DROP COLUMN IF EXISTS backfill_message_count,
    DROP COLUMN IF EXISTS backfill_updated_at;
