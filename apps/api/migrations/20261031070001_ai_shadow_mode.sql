-- +goose Up
-- +goose StatementBegin
-- ai-reply-flow-llm-design task 9.2: per-studio shadow mode. When true,
-- the new pipeline runs end-to-end for this studio's inbound messages
-- (debounce -> gates -> structured call -> tools (read-only only) ->
-- guards), writes an ai_turns row with outcome='shadow', but never
-- enqueues the outbound reply and never executes a side-effecting tool
-- call (CreateBooking) — the point is validating the pipeline against
-- real traffic before this studio's real customers see it. Defaults to
-- false so no existing studio's behavior changes until a superadmin
-- deliberately opts one in.
ALTER TABLE studios ADD COLUMN ai_shadow_mode BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS ai_shadow_mode;
-- +goose StatementEnd
