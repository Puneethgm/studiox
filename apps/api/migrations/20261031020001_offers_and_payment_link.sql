-- +goose Up
-- +goose StatementBegin
-- offers is ai-reply-flow-llm-design's structured-facts table (doc §4.4):
-- the model is handed an offer's price/terms directly instead of
-- inferring or retrieving them, and the guards capability checks a
-- generated reply's offer mentions against this table.
CREATE TABLE offers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    terms       TEXT NOT NULL DEFAULT '',
    price_sgd   INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_offers_studio ON offers(studio_id);

ALTER TABLE plans ADD COLUMN payment_link TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE plans DROP COLUMN IF EXISTS payment_link;
DROP TABLE IF EXISTS offers;
-- +goose StatementEnd
