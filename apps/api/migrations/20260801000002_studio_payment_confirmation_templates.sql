-- +goose Up
-- Customizable message templates sent right after a successful Stripe
-- trial/membership payment. Empty means "use the built-in default text".
ALTER TABLE studios
    ADD COLUMN trial_confirmation_message text NOT NULL DEFAULT '',
    ADD COLUMN membership_confirmation_message text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studios
    DROP COLUMN trial_confirmation_message,
    DROP COLUMN membership_confirmation_message;
