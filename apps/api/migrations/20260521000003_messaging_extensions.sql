-- +goose Up
-- +goose StatementBegin
ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind IN ('whatsapp_meta','instagram_meta','messenger_meta','x_dm','sms'));

ALTER TABLE message_templates ADD COLUMN attachments JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE trigger_links (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id   UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE trigger_link_clicks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    link_id    UUID NOT NULL REFERENCES trigger_links(id) ON DELETE CASCADE,
    lead_id    UUID REFERENCES leads(id) ON DELETE SET NULL,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trigger_link_clicks;
DROP TABLE IF EXISTS trigger_links;

ALTER TABLE message_templates DROP COLUMN IF EXISTS attachments;

ALTER TABLE channel_accounts DROP CONSTRAINT IF EXISTS channel_accounts_kind_check;
ALTER TABLE channel_accounts ADD CONSTRAINT channel_accounts_kind_check CHECK (kind IN ('whatsapp_meta','instagram_meta','messenger_meta','x_dm'));
-- +goose StatementEnd
