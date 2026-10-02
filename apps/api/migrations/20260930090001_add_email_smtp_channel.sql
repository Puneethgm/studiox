-- +goose Up

-- Allow 'email_smtp' as a channel kind + bsp — a studio's own outbound-email
-- account (host/port/user/password/from), stored the same way as every other
-- channel (JSON creds encrypted into access_token_enc, see
-- channels.SMTPCredentials). Outbound-only: unlike whatsapp/telegram/etc.
-- this never receives inbound webhooks and does not go through the
-- Sender-interface dispatch path — it's a credential vault a future feature
-- reads from, not an inbox channel.

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_kind_check
    CHECK (kind IN ('whatsapp_meta','whatsapp_web','instagram_meta','messenger_meta','x_dm','sms','google_ads','telegram','telegram_mtproto','email_smtp'));

ALTER TABLE channel_accounts
    DROP CONSTRAINT IF EXISTS channel_accounts_bsp_check;

ALTER TABLE channel_accounts
    ADD CONSTRAINT channel_accounts_bsp_check
    CHECK (bsp IN ('meta_direct','twilio','google','x_dm','baileys','telegram','telegram_mtproto','smtp'));

-- +goose Down

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
