-- +goose Up
ALTER TABLE users
    ADD COLUMN username             TEXT,
    ADD COLUMN first_name           TEXT,
    ADD COLUMN last_name            TEXT,
    ADD COLUMN role_id              UUID REFERENCES studio_roles(id),
    ADD COLUMN deleted_at           TIMESTAMPTZ,
    ADD COLUMN deleted_by           UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN must_reset_password BOOLEAN NOT NULL DEFAULT false;

-- Per-studio username uniqueness — partial index so the many existing
-- NULL-username rows (every super_admin/studio_admin today) never collide.
CREATE UNIQUE INDEX idx_users_studio_username ON users(studio_id, username)
    WHERE username IS NOT NULL;

-- Add 'studio_staff' as a third role value, subject to permission checks
-- (unlike super_admin/studio_admin, which always bypass them).
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('super_admin', 'studio_admin', 'studio_staff'));

ALTER TABLE users DROP CONSTRAINT users_check;
ALTER TABLE users ADD CONSTRAINT users_check
    CHECK ((role = 'super_admin' AND studio_id IS NULL)
        OR (role IN ('studio_admin', 'studio_staff') AND studio_id IS NOT NULL));

-- +goose Down
ALTER TABLE users DROP CONSTRAINT users_check;
ALTER TABLE users ADD CONSTRAINT users_check
    CHECK ((role = 'super_admin' AND studio_id IS NULL)
        OR (role = 'studio_admin' AND studio_id IS NOT NULL));

ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('super_admin', 'studio_admin'));

DROP INDEX idx_users_studio_username;

ALTER TABLE users
    DROP COLUMN username,
    DROP COLUMN first_name,
    DROP COLUMN last_name,
    DROP COLUMN role_id,
    DROP COLUMN deleted_at,
    DROP COLUMN deleted_by,
    DROP COLUMN must_reset_password;
