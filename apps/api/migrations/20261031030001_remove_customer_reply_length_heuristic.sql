-- +goose Up
-- +goose StatementBegin
-- ai-reply-flow-llm-design task 3.4 (doc §9: remove the ">=15 chars"
-- soft-failure rule). The mechanism itself (route.min_usable_length) was
-- already made configurable instead of hardcoded Go by the earlier
-- llm-gateway-and-key-management change — this is the actual removal:
-- customer_reply now returns structured, JSON-schema-validated output
-- (CompleteStructured), so a malformed/incomplete response already fails
-- to parse and falls through to the next provider on its own. A
-- character-count heuristic on top of real schema validation is
-- redundant for this feature specifically, so it's set to 0 (disabled)
-- here rather than left at the old default. Other features
-- (intent_classify/query_expand/rerank) are untouched — they still
-- return plain text and still benefit from the length guard.
UPDATE ai_feature_routes
SET route = jsonb_set(route, '{min_usable_length}', '0'::jsonb)
WHERE feature = 'customer_reply';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE ai_feature_routes
SET route = jsonb_set(route, '{min_usable_length}', '15'::jsonb)
WHERE feature = 'customer_reply';
-- +goose StatementEnd
