-- +goose Up
-- +goose StatementBegin
-- Prevents the same one-shot autocontact step (SourceRef
-- "lead:<id>:followup:0" or "lead:<id>:trial_followup:1day" — each meant
-- to be sent exactly once, ever, per lead) from producing two
-- outbound_jobs rows for the same conversation. Without this, a worker
-- that crashes or fails to persist "sent" between enqueueing the job and
-- marking the autocontact outbox row done re-processes that outbox row on
-- its next tick and enqueues a second, duplicate send. It also covers a
-- second, independent trigger for the same trial follow-up: both
-- autocontact_worker.go and AIWorker.scheduleTrialFollowup (ai_worker.go)
-- can enqueue "lead:<id>:trial_followup:1day" for the same lead.
--
-- Deliberately scoped narrow (source_kind = 'automation' AND source_ref
-- matching these two step patterns) rather than all source_kind =
-- 'automation' rows: other automation sends (booking/membership
-- shortcuts, decision-tree nodes, broadcasts, re-engagement nudges) may
-- legitimately fire more than once over a conversation's lifetime (e.g. a
-- customer asking to book a trial again weeks later), so deduplicating
-- those too would silently swallow real, intended sends. source_kind =
-- 'ai' is excluded entirely: there source_ref is a reusable label (model
-- name, trigger tag), not a per-instance key, and the same
-- (conversation_id, source_ref) legitimately repeats many times
-- (confirmed against existing data).
CREATE UNIQUE INDEX idx_outbound_jobs_autocontact_step_dedup
    ON outbound_jobs (conversation_id, source_ref)
    WHERE source_kind = 'automation'
      AND (source_ref LIKE 'lead:%:followup:%' OR source_ref LIKE 'lead:%:trial_followup:%');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX idx_outbound_jobs_autocontact_step_dedup;
-- +goose StatementEnd
