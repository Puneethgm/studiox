package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TodayAISpendUSDCents returns studioID's total LLM spend (in USD cents,
// rounded) since the start of today (UTC) — compared against
// studios.ai_daily_budget_cents by the budget gate (task 6.3). Reuses the
// existing llm_usage_logs.cost_usd column from the gateway's usage
// tracking rather than a second cost-tracking mechanism (design.md's
// explicit decision).
func (r *Repo) TodayAISpendUSDCents(ctx context.Context, studioID uuid.UUID) (int, error) {
	var costUSD float64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM llm_usage_logs
		WHERE studio_id = $1 AND created_at >= date_trunc('day', now())
	`, studioID).Scan(&costUSD)
	if err != nil {
		return 0, fmt.Errorf("today ai spend: %w", err)
	}
	return int(costUSD*100 + 0.5), nil
}

// TodayAIReplyCountForConversation returns how many AI-sourced outbound
// messages conversationID has received since the start of today (UTC) —
// the per-conversation daily reply cap (task 6.4).
func (r *Repo) TodayAIReplyCountForConversation(ctx context.Context, conversationID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM messages
		WHERE conversation_id = $1 AND source_kind = 'ai' AND direction = 'outbound'
		  AND sent_at >= date_trunc('day', now())
	`, conversationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("today ai reply count: %w", err)
	}
	return count, nil
}

// aiAfterHoursBehavior/aiDailyBudgetCents read the two gate-config
// columns added by this group's migration (task 6 intro).
func (r *Repo) aiAfterHoursBehavior(ctx context.Context, studioID uuid.UUID) (string, error) {
	var v string
	err := r.pool.QueryRow(ctx, `SELECT ai_after_hours_behavior FROM studios WHERE id = $1`, studioID).Scan(&v)
	if err != nil {
		return "allow", err
	}
	return v, nil
}

func (r *Repo) aiDailyBudgetCents(ctx context.Context, studioID uuid.UUID) (*int, error) {
	var v *int
	err := r.pool.QueryRow(ctx, `SELECT ai_daily_budget_cents FROM studios WHERE id = $1`, studioID).Scan(&v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// aiShadowMode reads the shadow-mode flag (task 9.2) added by this
// migration. Same lean direct-SQL-read pattern as the two gate-config
// readers above, rather than adding the column to the full Studio struct.
func (r *Repo) aiShadowMode(ctx context.Context, studioID uuid.UUID) (bool, error) {
	var v bool
	err := r.pool.QueryRow(ctx, `SELECT ai_shadow_mode FROM studios WHERE id = $1`, studioID).Scan(&v)
	if err != nil {
		return false, err
	}
	return v, nil
}

// AIActiveHoursConfig is the 4-column "AI Active Hours" gate config —
// read together in one query both by the gate check and by the Settings
// UI's GET endpoint, same lean direct-SQL pattern as the other
// gate-config readers above (exported, unlike those, since the Settings
// round-trip needs it too, not just the gate).
type AIActiveHoursConfig struct {
	Enabled  bool
	Start    string // "HH:MM"
	End      string // "HH:MM"
	Timezone string // IANA zone name
}

func (r *Repo) GetAIActiveHours(ctx context.Context, studioID uuid.UUID) (AIActiveHoursConfig, error) {
	var cfg AIActiveHoursConfig
	err := r.pool.QueryRow(ctx, `
		SELECT ai_active_hours_enabled,
		       to_char(ai_active_hours_start, 'HH24:MI'),
		       to_char(ai_active_hours_end, 'HH24:MI'),
		       ai_active_hours_timezone
		FROM studios WHERE id = $1
	`, studioID).Scan(&cfg.Enabled, &cfg.Start, &cfg.End, &cfg.Timezone)
	if err != nil {
		return AIActiveHoursConfig{}, fmt.Errorf("get ai active hours: %w", err)
	}
	return cfg, nil
}

// SetAIActiveHours updates the "AI Active Hours" gate config for a studio
// — the admin-settings counterpart to GetAIActiveHours above.
func (r *Repo) SetAIActiveHours(ctx context.Context, studioID uuid.UUID, enabled bool, start, end, timezone string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET ai_active_hours_enabled = $2, ai_active_hours_start = $3, ai_active_hours_end = $4,
		    ai_active_hours_timezone = $5, updated_at = now()
		WHERE id = $1
	`, studioID, enabled, start, end, timezone)
	if err != nil {
		return fmt.Errorf("set ai active hours: %w", err)
	}
	return nil
}

// SetAIShadowMode toggles shadow mode for a studio — used by the
// super-admin UI (task 9.2 / the admin-UI follow-up) and directly in
// tests.
func (r *Repo) SetAIShadowMode(ctx context.Context, studioID uuid.UUID, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE studios SET ai_shadow_mode = $2 WHERE id = $1`, studioID, enabled)
	if err != nil {
		return fmt.Errorf("set ai shadow mode: %w", err)
	}
	return nil
}

// AITurn is one row of ai_turns — the doc §8.9 per-reply measurement
// record. Written for both live replies (outcome = sent/guard_failed/
// all_providers_failed/handoff reasons) and shadow-mode runs
// (outcome = "shadow"), so the same table and metrics apply to both.
type AITurn struct {
	ConversationID   uuid.UUID
	StudioID         uuid.UUID
	PipelineVersion  string
	StructuredOutput any // marshaled to jsonb; nil if the structured call itself failed
	GuardOutcome     string
	ToolCalls        any // marshaled to jsonb; nil if no tool calls were made
	LatencyMS        *int
	CostUSD          *float64
	Outcome          string
}

// InsertAITurn records one ai_turns row. Best-effort: a failure to record
// the measurement must never abort the primary action (sending a reply,
// or in shadow mode, simply moving on) — logged by the caller, not
// retried here, matching this codebase's existing treatment of
// non-critical side effects (see internal/platform/audit's package doc).
func (r *Repo) InsertAITurn(ctx context.Context, t AITurn) error {
	structuredJSON, err := marshalOrNil(t.StructuredOutput)
	if err != nil {
		return fmt.Errorf("marshal ai turn structured output: %w", err)
	}
	toolCallsJSON, err := marshalOrNil(t.ToolCalls)
	if err != nil {
		return fmt.Errorf("marshal ai turn tool calls: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO ai_turns (conversation_id, studio_id, pipeline_version, structured_output,
			guard_outcome, tool_calls, latency_ms, cost_usd, outcome)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, t.ConversationID, t.StudioID, t.PipelineVersion, structuredJSON, t.GuardOutcome,
		toolCallsJSON, t.LatencyMS, t.CostUSD, t.Outcome)
	if err != nil {
		return fmt.Errorf("insert ai turn: %w", err)
	}
	return nil
}

func marshalOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

