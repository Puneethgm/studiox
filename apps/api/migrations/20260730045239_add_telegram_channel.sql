-- +goose Up

-- Allow 'telegram' as a channel kind + bsp, and 'telegram_chat_id' as a
-- contact identity kind. CHECK constraints must be dropped and recreated.

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_kind_check
    CHECK (kind IN ('whatsapp_meta','whatsapp_web','instagram_meta','messenger_meta','x_dm','sms','google_ads','telegram'));

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_bsp_check
    CHECK (bsp IN ('meta_direct','twilio','google','x_dm','baileys','telegram'));

ALTER TABLE contact_identities
    DROP CONSTRAINT IF EXISTS contact_identities_kind_check;

ALTER TABLE contact_identities
    ADD CONSTRAINT contact_identities_kind_check
    CHECK (kind IN ('phone','email','ig_psid','fb_psid','x_id','telegram_chat_id'));

-- +goose Down

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

ALTER TABLE contact_identities
    DROP CONSTRAINT IF EXISTS contact_identities_kind_check;

ALTER TABLE contact_identities
    ADD CONSTRAINT contact_identities_kind_check
    CHECK (kind IN ('phone','email','ig_psid','fb_psid','x_id'));
