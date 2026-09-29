-- +goose Up
-- Per-studio toggle: when false, SendTrialPaymentLink skips Stripe checkout
-- entirely and instead sends a holding message + flags the lead for manual
-- staff follow-up. Defaults true so every existing studio keeps today's
-- behavior (collect payment via the trial link) unchanged.
ALTER TABLE studios ADD COLUMN trial_payment_enabled BOOLEAN NOT NULL DEFAULT true;

-- Reversible marker set on a lead when trial payment collection is
-- disabled and they hit the trial-booking flow — surfaced in the Leads
-- list/detail page so staff know to reach out manually. Cleared
-- automatically the next time a studio_user (human) sends that lead a
-- message.
ALTER TABLE leads ADD COLUMN needs_manual_followup BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE studios DROP COLUMN trial_payment_enabled;
ALTER TABLE leads DROP COLUMN needs_manual_followup;
