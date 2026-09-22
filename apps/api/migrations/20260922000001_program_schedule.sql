-- +goose Up
ALTER TABLE studios ADD COLUMN program_start_date DATE;

-- One parsed session per (studio, week, day-of-week) — replaced wholesale
-- whenever the source document is re-parsed, same pattern as
-- studio_knowledge_chunks.
CREATE TABLE studio_program_sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    studio_id     UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    week_number   INT NOT NULL,
    day_of_week   INT NOT NULL, -- ISO: Monday=1 .. Sunday=7
    day_name      TEXT NOT NULL,
    session_name  TEXT NOT NULL,
    progression   TEXT NOT NULL DEFAULT '',
    key_focus     TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (studio_id, week_number, day_of_week)
);

-- +goose Down
DROP TABLE studio_program_sessions;
ALTER TABLE studios DROP COLUMN program_start_date;
