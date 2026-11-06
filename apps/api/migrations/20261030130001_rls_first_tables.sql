-- +goose Up
-- +goose StatementBegin
-- platform-security-hardening task 7.1 — RLS on the first 2 highest-
-- risk studio-scoped tables (leads, messages), alongside (not
-- replacing) the existing WHERE-clause filtering every repo method
-- already does.
--
-- IMPORTANT, deliberate safety decision: these policies are FAIL-OPEN
-- when the `app.current_studio_id` session variable is unset — i.e.
-- every existing query in this codebase, none of which currently sets
-- that variable, keeps working exactly as before. This is intentional:
-- this codebase has no per-request transaction wrapping today (most
-- repo methods run bare `pool.Query`/`pool.Exec` calls against a shared
-- connection pool, not `SET LOCAL` inside a transaction scoped to one
-- request), so forcing RLS to deny-by-default right now would break
-- every single query in the app, not just cross-studio ones — the
-- blast radius of getting this wrong live is "the entire product stops
-- working," which is categorically worse than the isolation gap this
-- task closes. The isolation guarantee IS real and IS enforced the
-- moment a caller sets `app.current_studio_id` (see
-- internal/studios/rls.go's WithStudioScope and its test) — it's just
-- opt-in per-connection until every call site is migrated to set it,
-- which is the honest scope of this pass (see tasks.md 7.1's own note).
-- `app.bypass_rls = 'true'` is the explicit super_admin escape hatch,
-- usable even once a caller opts in.
-- FORCE is required in addition to ENABLE: without it, Postgres exempts
-- the table owner (the app's own migration/DB user) from every policy,
-- which would make this a no-op against exactly the connection this app
-- actually uses.
ALTER TABLE leads ENABLE ROW LEVEL SECURITY;
ALTER TABLE leads FORCE ROW LEVEL SECURITY;
CREATE POLICY leads_studio_isolation ON leads
  USING (
    current_setting('app.current_studio_id', true) IS NULL
    OR current_setting('app.bypass_rls', true) = 'true'
    OR studio_id = current_setting('app.current_studio_id', true)::uuid
  );

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
CREATE POLICY messages_studio_isolation ON messages
  USING (
    current_setting('app.current_studio_id', true) IS NULL
    OR current_setting('app.bypass_rls', true) = 'true'
    OR studio_id = current_setting('app.current_studio_id', true)::uuid
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS messages_studio_isolation ON messages;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS leads_studio_isolation ON leads;
ALTER TABLE leads DISABLE ROW LEVEL SECURITY;
-- +goose StatementEnd
