-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- A studio is the tenant of the platform. Each studio has its own admins,
-- campaigns, leads, branding, and Google Sheet target.
CREATE TABLE studios (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    brand_color  TEXT NOT NULL DEFAULT '#7c3aed',
    logo_url     TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_studios_active ON studios(active) WHERE active;

-- Users: super_admin (studio_id NULL — manages everything), studio_admin
-- (studio_id set — scoped to that studio).
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id       UUID REFERENCES studios(id) ON DELETE RESTRICT,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('super_admin','studio_admin')),
    -- A super_admin must have NULL studio_id; a studio_admin must have a studio_id.
    CHECK ((role = 'super_admin' AND studio_id IS NULL)
        OR (role = 'studio_admin' AND studio_id IS NOT NULL)),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_studio ON users(studio_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS studios;
-- +goose StatementEnd
