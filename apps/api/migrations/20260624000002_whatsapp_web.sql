-- +goose Up

-- Allow whatsapp_web kind and baileys BSP in channel_accounts.
-- CHECK constraints on individual columns must be dropped and recreated.

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_kind_check
    CHECK (kind IN ('whatsapp_meta','whatsapp_web','instagram_meta','messenger_meta','x_dm','sms','google_ads'));

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_bsp_check
    CHECK (bsp IN ('meta_direct','twilio','google','x_dm','baileys'));

-- +goose Down

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_kind_check
    CHECK (kind IN ('whatsapp_meta','instagram_meta','messenger_meta','x_dm','sms','google_ads'));

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_bsp_check
    CHECK (bsp IN ('meta_direct','twilio','google','x_dm'));
