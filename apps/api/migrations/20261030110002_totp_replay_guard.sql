-- +goose Up
-- Rejects reusing the same TOTP code within its still-valid window (task
-- 2.3's "a 30-second-old code already used once is rejected on reuse") —
-- tracks the last accepted 30-second step index per user.
ALTER TABLE user_totp ADD COLUMN IF NOT EXISTS last_used_step BIGINT;

-- +goose Down
ALTER TABLE user_totp DROP COLUMN IF EXISTS last_used_step;
