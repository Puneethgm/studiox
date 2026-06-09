-- +goose Up
-- Fix users table foreign key to allow cascade delete
ALTER TABLE users DROP CONSTRAINT users_studio_id_fkey;
ALTER TABLE users
  ADD CONSTRAINT users_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE;

-- +goose Down
-- Revert to ON DELETE RESTRICT
ALTER TABLE users DROP CONSTRAINT users_studio_id_fkey;
ALTER TABLE users
  ADD CONSTRAINT users_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE RESTRICT;
