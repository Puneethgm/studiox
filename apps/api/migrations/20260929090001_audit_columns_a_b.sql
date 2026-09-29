-- +goose Up
-- Audit-trail retrofit (Phase 2 of add-studio-rbac-and-audit-trail),
-- Category A (full audit: user-edited via UI) and Category B (created_by/
-- updated_by only, immutable once written). See design.md decision 7 for
-- the full table categorization and the four tables below that already had
-- half these columns before this migration (checked live via \d <table>,
-- not guessed): campaigns/automation_rules/crm_providers had created_by
-- already; ai_task_configs had updated_by already. This migration adds only
-- the missing half for those four, to avoid a duplicate-column error.

-- Category A, full set (created_by, updated_by, deleted_by, deleted_at) —
-- tables with no pre-existing audit column at all.
ALTER TABLE studios
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE leads
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE channel_accounts
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE message_templates
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE trigger_links
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE decision_trees
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE tree_nodes
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_knowledge_chunks
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE crm_connections
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE reviews
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE social_posts
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_ai_models
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_followup_steps
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_sheets_settings
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_external_leads_sheet_settings
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE studio_program_sessions
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE plans
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- Category A, missing-half only — created_by already existed.
ALTER TABLE campaigns
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE automation_rules
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE crm_providers
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- Category A, missing-half only — updated_by already existed. Not found
-- during the original design pass; added here once the live-schema check
-- for this migration turned it up (see design.md decision 7's correction).
ALTER TABLE ai_task_configs
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- Category A, missing-half only — deleted_at/deleted_by already existed
-- (added by this same change's Phase 1, for teammate deactivation).
ALTER TABLE users
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Category B: created_by/updated_by only — no delete action exists for
-- any of these (append-only from the app's perspective).
ALTER TABLE conversations
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE messages
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE outbound_jobs
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE ai_suggestions
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE message_analyses
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE external_sheet_import_log
    ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE external_sheet_import_log DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE message_analyses DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE ai_suggestions DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE outbound_jobs DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE messages DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE conversations DROP COLUMN created_by, DROP COLUMN updated_by;

ALTER TABLE users DROP COLUMN created_by, DROP COLUMN updated_by;
ALTER TABLE ai_task_configs DROP COLUMN created_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE crm_providers DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE automation_rules DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE campaigns DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;

ALTER TABLE plans DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_program_sessions DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_external_leads_sheet_settings DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_sheets_settings DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_followup_steps DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_ai_models DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE social_posts DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE reviews DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE crm_connections DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studio_knowledge_chunks DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE tree_nodes DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE decision_trees DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE trigger_links DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE message_templates DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE channel_accounts DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE leads DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE studios DROP COLUMN created_by, DROP COLUMN updated_by, DROP COLUMN deleted_by, DROP COLUMN deleted_at;
