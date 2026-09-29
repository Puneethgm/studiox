-- +goose Up
-- Reverts 20260929120001_trial_payment_toggle.sql's studios.
-- trial_payment_enabled column — replaced by reusing the existing
-- Active/Inactive toggle on the plan literally named "Trial" (see
-- internal/messaging.Repo.IsTrialPlanActive). leads.needs_manual_followup
-- stays; that flag is still set/cleared the same way, just gated on the
-- Trial plan's own active state now instead of a separate switch.
ALTER TABLE studios DROP COLUMN trial_payment_enabled;

-- +goose Down
ALTER TABLE studios ADD COLUMN trial_payment_enabled BOOLEAN NOT NULL DEFAULT true;
