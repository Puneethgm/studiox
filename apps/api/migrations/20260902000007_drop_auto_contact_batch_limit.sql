-- +goose Up
-- Reverting auto_contact_batch_limit from 20260902000004 — leads beyond the
-- cap were permanently silenced (never auto-contacted), which isn't what's
-- wanted. Every imported lead should eventually get contacted; the existing
-- whatsapp_send_spacing_seconds already paces sends safely regardless of
-- batch size, so a separate volume cap isn't needed.
ALTER TABLE studio_external_leads_sheet_settings DROP COLUMN auto_contact_batch_limit;

-- +goose Down
ALTER TABLE studio_external_leads_sheet_settings ADD COLUMN auto_contact_batch_limit INT NOT NULL DEFAULT 0;
