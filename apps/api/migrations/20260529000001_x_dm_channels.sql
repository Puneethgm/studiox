-- +goose Up
-- Add X/Twitter to channel_accounts_bsp_check
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text, 'google'::text, 'x'::text, 'x_dm'::text]));

-- +goose Down
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text, 'google'::text]));
