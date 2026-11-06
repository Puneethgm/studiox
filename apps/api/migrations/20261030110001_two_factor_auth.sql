-- +goose Up
-- platform-security-hardening task 2.1. TOTP secret is envelope-
-- encrypted with the existing secrets.Cipher (same pattern as every
-- other at-rest secret in this app); recovery codes are bcrypt hashes
-- (one-way — they're shown once at enrollment and never need to be
-- read back, unlike the TOTP secret which must be decrypted to verify
-- a code).
CREATE TABLE IF NOT EXISTS user_totp (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_ciphertext TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    enrolled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user_id ON user_recovery_codes (user_id);

-- Remember-this-browser grants (studio_admin/studio_staff only, 14 days,
-- never super_admin) — see design.md decision 1. A Postgres table (not
-- just a signed cookie) so it can be revoked on logout-all/password
-- change the same way sessions are.
CREATE TABLE IF NOT EXISTS remember_browser_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_remember_browser_tokens_user_id ON remember_browser_tokens (user_id, revoked_at);

-- +goose Down
DROP TABLE IF EXISTS remember_browser_tokens;
DROP TABLE IF EXISTS user_recovery_codes;
DROP TABLE IF EXISTS user_totp;
