-- +goose Up
-- Audit-trail retrofit (Phase 2 of add-studio-rbac-and-audit-trail),
-- Category C: system/log tables with no human actor, ever. Columns are
-- added per explicit instruction to cover every table, even though they
-- are expected to stay NULL in practice for all of these (see design.md
-- decision 7 and the spec's "system-initiated writes never fabricate a
-- human actor" requirement) — no application code will be changed to
-- populate them.

ALTER TABLE outbox
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE llm_usage_logs
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE processed_stripe_events
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE trigger_link_clicks
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE glofox_first_session_log
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE automation_runs
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE onboarding_tokens
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE contact_identities
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE user_subscriptions
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE studio_subscriptions
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE crm_operations
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- platform_settings: updated_by only — it's a key/value settings table
-- (upserted), edited only by a super_admin; "created_by" doesn't add
-- anything distinct from "updated_by" for a row that's always upserted,
-- and there's no delete concept.
ALTER TABLE platform_settings
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE platform_settings DROP COLUMN updated_by;
ALTER TABLE crm_operations DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE studio_subscriptions DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE user_subscriptions DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE contact_identities DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE onboarding_tokens DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE automation_runs DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE glofox_first_session_log DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE trigger_link_clicks DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE processed_stripe_events DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE llm_usage_logs DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE outbox DROP COLUMN created_by, DROP COLUMN updated_by;
