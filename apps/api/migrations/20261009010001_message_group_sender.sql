-- +goose Up
-- WhatsApp group chats have many senders behind one conversation. Without
-- this, every message in a group looked like it came from the same
-- "Contact" (the group itself) — no way to tell which member sent which
-- message. Only ever populated for group messages; null for 1:1 DMs.
ALTER TABLE messages
    ADD COLUMN sender_jid TEXT,
    ADD COLUMN sender_name TEXT;

-- +goose Down
ALTER TABLE messages
    DROP COLUMN sender_jid,
    DROP COLUMN sender_name;
