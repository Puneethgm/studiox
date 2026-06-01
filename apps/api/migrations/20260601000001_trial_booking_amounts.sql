-- +goose Up
-- +goose StatementBegin
ALTER TABLE studios
ADD COLUMN trial_amount_sgd INTEGER NOT NULL DEFAULT 2500, -- stored in cents (e.g. 2500 = 25.00 SGD)
ADD COLUMN trial_amount_inr INTEGER NOT NULL DEFAULT 150000, -- stored in paise (e.g. 150000 = 1500.00 INR)
ADD COLUMN trial_amount_usd INTEGER NOT NULL DEFAULT 2000; -- stored in cents (e.g. 2000 = 20.00 USD)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios
DROP COLUMN trial_amount_sgd,
DROP COLUMN trial_amount_inr,
DROP COLUMN trial_amount_usd;
-- +goose StatementEnd
