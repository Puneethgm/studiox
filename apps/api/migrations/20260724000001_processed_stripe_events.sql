-- +goose Up
-- Stripe delivers webhook events at-least-once and occasionally redelivers
-- the same event even after a 200 response. Without a dedup guard, every
-- redelivery of checkout.session.completed re-ran handleCheckoutComplete
-- and enqueued a duplicate WhatsApp confirmation message.
CREATE TABLE IF NOT EXISTS processed_stripe_events (
    event_id   text PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS processed_stripe_events;
