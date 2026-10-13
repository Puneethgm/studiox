-- +goose Up
-- +goose StatementBegin
-- A single currency per studio, not a column per currency (see the
-- abandoned trial_amount_inr/trial_amount_usd columns from
-- 20260601000001_trial_booking_amounts.sql, which were added but never
-- wired up anywhere). trial_amount_sgd and membership_plans.price_sgd keep
-- their existing column names (renaming them everywhere would be a much
-- larger, riskier change) but are now interpreted as "amount in cents, in
-- whatever currency this studio is configured for" rather than strictly
-- SGD once a studio sets a non-SGD currency.
ALTER TABLE studios
ADD COLUMN currency TEXT NOT NULL DEFAULT 'SGD';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios
DROP COLUMN currency;
-- +goose StatementEnd
