-- +goose Up
-- How many times a broadcast recipient has been automatically put back in the queue
-- after its message failed for a temporary reason (network error, daily limit hit,
-- channel offline). Caps automatic retries so a permanently failing recipient can't loop.
ALTER TABLE broadcast_campaign_recipients ADD COLUMN retry_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE broadcast_campaign_recipients DROP COLUMN retry_count;
