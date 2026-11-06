-- +goose Up
-- platform-security-hardening task 1.1. The JWT's own `jti` rotates on
-- every authenticated request (see TokenIssuer.Refresh) — too churny to
-- be the durable session identity. Claims.SessionID is a new, separate
-- claim generated once at Issue (login/password-change) and carried
-- forward unchanged by every Refresh, same pattern already used for
-- CSRFToken/SessionMaxExpiresAt. This table is keyed on that stable ID.
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device TEXT NOT NULL DEFAULT '',
    browser TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    two_fa_passed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    revoked_reason TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id, revoked_at);

-- +goose Down
DROP TABLE IF EXISTS sessions;
