-- +goose Up
-- +goose StatementBegin
-- Set when a message was deleted — real WhatsApp "delete for everyone" via
-- Baileys (apps/wa-web's SessionManager.deleteMessage) for whatsapp_web
-- conversations today; other channels don't support an equivalent delete
-- yet. body/attachments are cleared at the same time the content actually
-- becomes inaccessible on WhatsApp's own servers too, not just hidden here
-- — mirrors the real protocol-level effect rather than a soft-hide that
-- would leave deleted content sitting in this database indefinitely.
ALTER TABLE messages
ADD COLUMN deleted_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE messages
DROP COLUMN deleted_at;
-- +goose StatementEnd
