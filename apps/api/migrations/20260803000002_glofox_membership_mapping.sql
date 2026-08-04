-- +goose Up
-- Glofox membership/plan-code mapping so a real Stripe trial/membership
-- payment can create an actual credit-pack/membership purchase in Glofox
-- (not just a bare lead record). Studio admin looks these up once via
-- Glofox's own dashboard/API and enters them here.
ALTER TABLE studios
    ADD COLUMN trial_glofox_membership_id text NOT NULL DEFAULT '',
    ADD COLUMN trial_glofox_plan_code text NOT NULL DEFAULT '',
    ADD COLUMN membership_glofox_membership_id text NOT NULL DEFAULT '',
    ADD COLUMN membership_glofox_plan_code text NOT NULL DEFAULT '';

-- Audit trail: the invoice_id Glofox returns from a successful purchase call.
ALTER TABLE leads
    ADD COLUMN glofox_invoice_id text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studios
    DROP COLUMN trial_glofox_membership_id,
    DROP COLUMN trial_glofox_plan_code,
    DROP COLUMN membership_glofox_membership_id,
    DROP COLUMN membership_glofox_plan_code;

ALTER TABLE leads
    DROP COLUMN glofox_invoice_id;
