-- +goose Up
-- +goose StatementBegin
-- 1. Ensure the google credentials columns are NOT NULL and default to ''
UPDATE studios SET google_client_id = '' WHERE google_client_id IS NULL;
UPDATE studios SET google_client_secret = '' WHERE google_client_secret IS NULL;
UPDATE studios SET google_developer_token = '' WHERE google_developer_token IS NULL;

ALTER TABLE studios ALTER COLUMN google_client_id SET DEFAULT '';
ALTER TABLE studios ALTER COLUMN google_client_id SET NOT NULL;

ALTER TABLE studios ALTER COLUMN google_client_secret SET DEFAULT '';
ALTER TABLE studios ALTER COLUMN google_client_secret SET NOT NULL;

ALTER TABLE studios ALTER COLUMN google_developer_token SET DEFAULT '';
ALTER TABLE studios ALTER COLUMN google_developer_token SET NOT NULL;

-- 2. Update channel_accounts check constraints if not already matching
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text, 'google'::text]));

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind = ANY (ARRAY['whatsapp_meta'::text, 'instagram_meta'::text, 'messenger_meta'::text, 'x_dm'::text, 'sms'::text, 'google_ads'::text]));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios ALTER COLUMN google_client_id DROP NOT NULL;
ALTER TABLE studios ALTER COLUMN google_client_id DROP DEFAULT;

ALTER TABLE studios ALTER COLUMN google_client_secret DROP NOT NULL;
ALTER TABLE studios ALTER COLUMN google_client_secret DROP DEFAULT;

ALTER TABLE studios ALTER COLUMN google_developer_token DROP NOT NULL;
ALTER TABLE studios ALTER COLUMN google_developer_token DROP DEFAULT;

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_bsp_check CHECK (bsp = ANY (ARRAY['meta_direct'::text, 'twilio'::text]));

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind = ANY (ARRAY['whatsapp_meta'::text, 'instagram_meta'::text, 'messenger_meta'::text, 'x_dm'::text, 'sms'::text]));
-- +goose StatementEnd
