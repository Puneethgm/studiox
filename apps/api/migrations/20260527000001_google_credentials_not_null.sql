-- +goose Up
-- +goose StatementBegin
-- 1. Ensure the google credentials columns are added with NOT NULL and default to ''
ALTER TABLE studios ADD COLUMN IF NOT EXISTS google_client_id TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN IF NOT EXISTS google_client_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE studios ADD COLUMN IF NOT EXISTS google_developer_token TEXT NOT NULL DEFAULT '';

-- 2. Update channel_accounts check constraints if not already matching
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text, 'google'::text]));
`
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind = ANY (ARRAY['whatsapp_meta'::text, 'instagram_meta'::text, 'messenger_meta'::text, 'x_dm'::text, 'sms'::text, 'google_ads'::text]));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS google_client_id;
ALTER TABLE studios DROP COLUMN IF EXISTS google_client_secret;
ALTER TABLE studios DROP COLUMN IF EXISTS google_developer_token;

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text]));

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind = ANY (ARRAY['whatsapp_meta'::text, 'instagram_meta'::text, 'messenger_meta'::text, 'x_dm'::text, 'sms'::text]));
-- +goose StatementEnd
