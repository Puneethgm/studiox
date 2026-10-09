-- +goose Up
-- Remembers the last text/template an admin used for Pipeline's Cold-column
-- "Re-engage" action, to pre-fill the modal next time.
CREATE TABLE studio_reengage_prefs (
    studio_id   UUID PRIMARY KEY REFERENCES studios(id) ON DELETE CASCADE,
    message     TEXT NOT NULL DEFAULT '',
    template_id UUID REFERENCES message_templates(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE studio_reengage_prefs;
