-- +goose Up
-- A step can now point at a saved message_templates row; message_template
-- stays as a fallback.
ALTER TABLE studio_followup_steps
    ADD COLUMN template_id UUID REFERENCES message_templates(id) ON DELETE SET NULL;

ALTER TABLE studio_followup_steps
    ALTER COLUMN message_template DROP NOT NULL;

-- +goose Down
ALTER TABLE studio_followup_steps
    ALTER COLUMN message_template SET NOT NULL;

ALTER TABLE studio_followup_steps
    DROP COLUMN template_id;
