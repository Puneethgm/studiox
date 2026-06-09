-- +goose Up
-- Drop existing foreign key constraints with ON DELETE RESTRICT
-- and recreate them with ON DELETE CASCADE for proper account deletion

-- Leads table: drop the restrict constraint and recreate with cascade
ALTER TABLE leads DROP CONSTRAINT leads_studio_id_fkey;
ALTER TABLE leads
  ADD CONSTRAINT leads_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE;

-- Campaigns table: drop the restrict constraint and recreate with cascade
ALTER TABLE campaigns DROP CONSTRAINT campaigns_studio_id_fkey;
ALTER TABLE campaigns
  ADD CONSTRAINT campaigns_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE;

-- +goose Down
-- Revert to ON DELETE RESTRICT if rolling back
ALTER TABLE leads DROP CONSTRAINT leads_studio_id_fkey;
ALTER TABLE leads
  ADD CONSTRAINT leads_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE RESTRICT;

ALTER TABLE campaigns DROP CONSTRAINT campaigns_studio_id_fkey;
ALTER TABLE campaigns
  ADD CONSTRAINT campaigns_studio_id_fkey
  FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE RESTRICT;
