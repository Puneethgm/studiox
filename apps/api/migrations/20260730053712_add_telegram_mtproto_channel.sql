-- +goose Up

-- Allow 'telegram_mtproto' as a channel kind + bsp — a QR-linked personal
-- Telegram account (via tg-web/teleproto), distinct from the bot-token
-- 'telegram' kind. Shares 'telegram_chat_id' as its contact identity kind
-- since both address a Telegram chat by the same numeric id.

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_kind_check
    CHECK (kind IN ('whatsapp_meta','whatsapp_web','instagram_meta','messenger_meta','x_dm','sms','google_ads','telegram','telegram_mtproto'));

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_bsp_check
    CHECK (bsp IN ('meta_direct','twilio','google','x_dm','baileys','telegram','telegram_mtproto'));

-- +goose Down

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
