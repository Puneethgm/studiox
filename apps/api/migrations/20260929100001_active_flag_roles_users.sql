-- +goose Up
-- Reversible active/inactive toggle for studio_roles and users — distinct
-- from users.deleted_at (a permanent, one-way removal). Deactivating a role
-- blocks login for every user assigned to it; deactivating a user blocks
-- just that user. Both default true so every existing row stays usable.
ALTER TABLE studio_roles ADD COLUMN active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN active BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE studio_roles DROP COLUMN active;
ALTER TABLE users DROP COLUMN active;
