-- +goose Up
-- Optional gender and date of birth, collected on the pre-payment trial
-- details page before checkout. Used to send real values to Glofox instead
-- of the neutral birth-date placeholder.
ALTER TABLE leads
    ADD COLUMN gender text NOT NULL DEFAULT '',
    ADD COLUMN date_of_birth date;

-- +goose Down
ALTER TABLE leads
    DROP COLUMN gender,
    DROP COLUMN date_of_birth;
