-- +goose Up
UPDATE studios SET managed_by_1hero = true;

-- +goose Down
-- No-op or revert to default false if desired:
-- UPDATE studios SET managed_by_1hero = false;
