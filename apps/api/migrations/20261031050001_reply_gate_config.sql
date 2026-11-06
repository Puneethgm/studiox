-- +goose Up
-- +goose StatementBegin
-- ai-reply-flow-llm-design task 6 config: the two gates that need
-- per-studio configuration a studio doesn't already have. Both default
-- to "no behavior change" (after_hours_behavior='allow', budget=NULL/
-- unlimited) so existing studios aren't silently affected until a
-- superadmin deliberately opts in.
ALTER TABLE studios ADD COLUMN ai_after_hours_behavior TEXT NOT NULL DEFAULT 'allow';
ALTER TABLE studios ADD COLUMN ai_daily_budget_cents INTEGER;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE studios DROP COLUMN IF EXISTS ai_daily_budget_cents;
ALTER TABLE studios DROP COLUMN IF EXISTS ai_after_hours_behavior;
-- +goose StatementEnd
