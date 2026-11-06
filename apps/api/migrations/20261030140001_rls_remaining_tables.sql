-- +goose Up
-- +goose StatementBegin
-- platform-security-hardening task 9.1 — extends task 7.1's RLS
-- pattern (leads, messages) to every other studio-scoped table (every
-- table with a `studio_id` column, found via a live catalog query
-- against this schema). Same fail-open-by-default, FORCE-enabled,
-- bypass-via-app.bypass_rls design as 7.1 — see
-- migrations/20261030130001_rls_first_tables.sql's doc comment for the
-- full reasoning (no per-request transaction wrapping exists yet in
-- this codebase's repo methods, so deny-by-default right now would
-- break every query in the app, not just cross-studio ones).
--
-- Looping via DO/EXECUTE instead of 41 nearly-identical CREATE POLICY
-- statements — every one of these tables gets the exact same policy
-- shape, so a loop keeps this migration reviewable and avoids a
-- copy-paste mistake in any single one.
DO $$
DECLARE
  tbl TEXT;
  tables TEXT[] := ARRAY[
    'account_archives', 'ai_feedback_corrections', 'ai_suggestions', 'audit_logs',
    'automation_rules', 'automation_runs', 'broadcast_campaigns', 'broadcast_import_staging',
    'broadcast_lists', 'campaigns', 'channel_accounts', 'contact_identities', 'conversations',
    'crm_connections', 'decision_trees', 'external_sheet_import_log', 'glofox_attendance',
    'glofox_attendance_settings', 'glofox_first_session_log', 'knowledge_gaps', 'llm_usage_logs',
    'message_analyses', 'message_templates', 'outbound_jobs', 'plans', 'platform_alerts',
    'retained_payment_records', 'security_audit_log', 'social_posts', 'studio_deletion_requests',
    'studio_external_leads_sheet_settings', 'studio_followup_steps', 'studio_knowledge_chunks',
    'studio_program_sessions', 'studio_reengage_prefs', 'studio_roles', 'studio_sheets_settings',
    'studio_subscriptions', 'trigger_links', 'user_subscriptions'
    -- 'users' is deliberately excluded: its studio_id is NULLABLE
    -- (super_admin/studio_admin rows have no studio_id at all), and
    -- this policy shape (`studio_id = current_setting(...)::uuid`)
    -- would need a NULL-aware variant to not misbehave for those rows
    -- — flagged as a follow-up, not silently included and broken.
  ];
BEGIN
  FOREACH tbl IN ARRAY tables LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', tbl);
    EXECUTE format(
      'CREATE POLICY %I ON %I USING (
         current_setting(''app.current_studio_id'', true) IS NULL
         OR current_setting(''app.bypass_rls'', true) = ''true''
         OR studio_id = current_setting(''app.current_studio_id'', true)::uuid
       )', tbl || '_studio_isolation', tbl
    );
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
  tbl TEXT;
  tables TEXT[] := ARRAY[
    'account_archives', 'ai_feedback_corrections', 'ai_suggestions', 'audit_logs',
    'automation_rules', 'automation_runs', 'broadcast_campaigns', 'broadcast_import_staging',
    'broadcast_lists', 'campaigns', 'channel_accounts', 'contact_identities', 'conversations',
    'crm_connections', 'decision_trees', 'external_sheet_import_log', 'glofox_attendance',
    'glofox_attendance_settings', 'glofox_first_session_log', 'knowledge_gaps', 'llm_usage_logs',
    'message_analyses', 'message_templates', 'outbound_jobs', 'plans', 'platform_alerts',
    'retained_payment_records', 'security_audit_log', 'social_posts', 'studio_deletion_requests',
    'studio_external_leads_sheet_settings', 'studio_followup_steps', 'studio_knowledge_chunks',
    'studio_program_sessions', 'studio_reengage_prefs', 'studio_roles', 'studio_sheets_settings',
    'studio_subscriptions', 'trigger_links', 'user_subscriptions'
  ];
BEGIN
  FOREACH tbl IN ARRAY tables LOOP
    EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_studio_isolation', tbl);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', tbl);
  END LOOP;
END $$;
-- +goose StatementEnd
