-- +goose Up
--
-- Phase 3 of the CRM framework (see 20260912000001_crm_framework.sql):
-- token-exchange auth, for CRMs like Mindbody whose real API needs more than
-- one static header — it needs a short-lived bearer token obtained by
-- POSTing static credentials to a login endpoint, refreshed on expiry, ON
-- TOP OF the same static headers still going out on every call. Static
-- bearer/api_key/basic auth (Phase 1) can't express this; this migration
-- adds a fourth auth_type plus the columns that describe a provider's login
-- step. All new columns are only read when auth_type = 'token_exchange';
-- every existing provider (Glofox api_key, Mindbody api_key) is untouched.
ALTER TABLE crm_providers
    DROP CONSTRAINT crm_providers_auth_type_check,
    ADD CONSTRAINT crm_providers_auth_type_check
        CHECK (auth_type IN ('bearer','api_key','basic','token_exchange'));

ALTER TABLE crm_providers
    -- Where to POST for a token, relative to base_url, e.g. "/usertoken/issue".
    ADD COLUMN token_login_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN token_login_method TEXT NOT NULL DEFAULT 'POST',
    -- {"Username":"const:Siteowner","Password":"api_key"} — the login
    -- request body. Each value is either a literal (prefixed "const:") or
    -- the key of one of this provider's auth_field_defs, whose per-studio
    -- credential value gets substituted in. Keeps CRM-specific constants
    -- (like Mindbody requiring the literal username "Siteowner") in data,
    -- not in Go code.
    ADD COLUMN token_login_body_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Dot-path into the login response for the token, e.g. "AccessToken".
    ADD COLUMN token_response_path TEXT NOT NULL DEFAULT '',
    -- Dot-path into the login response for the token's lifetime in seconds
    -- (e.g. Mindbody's "ExpiresIn"). Empty means use token_expiry_seconds
    -- instead, for a login response that doesn't report its own expiry.
    ADD COLUMN token_expiry_path TEXT NOT NULL DEFAULT '',
    ADD COLUMN token_expiry_seconds INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE crm_providers
    DROP COLUMN token_login_path,
    DROP COLUMN token_login_method,
    DROP COLUMN token_login_body_mapping,
    DROP COLUMN token_response_path,
    DROP COLUMN token_expiry_path,
    DROP COLUMN token_expiry_seconds;

ALTER TABLE crm_providers
    DROP CONSTRAINT crm_providers_auth_type_check,
    ADD CONSTRAINT crm_providers_auth_type_check
        CHECK (auth_type IN ('bearer','api_key','basic'));
