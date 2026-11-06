package llmgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo is llmgateway's own data access — ai_feature_routes (routing
// config), llm_usage_logs (per-call attribution/cost) and platform_alerts
// (failure/spend conditions). These tables are read/written directly here
// rather than via internal/messaging.Repo to avoid an import cycle
// (messaging calls into llmgateway, so llmgateway cannot import messaging)
// — see design.md Decision 1. Usage/alert rows written here land in the
// exact same tables internal/messaging.Repo's LogLLMUsage/RecordAlert
// write to; there is one set of tables, two repos that can write them
// during the migration, until the old call sites are deleted (Phase 3).
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// Route is one feature's routing config: which providers to try and in
// what order, plus per-provider call settings. Which MODEL(S) within a
// provider to try is NOT part of this — that stays the existing
// llm.Repo.EnabledModels/platform_ai_models system, superadmin-managed on
// the same AI Provider Settings page, unchanged by this gateway.
type Route struct {
	Providers       []string           `json:"providers"`
	MaxTokens       map[string]int     `json:"max_tokens,omitempty"`
	Temperature     map[string]float64 `json:"temperature,omitempty"`
	MinUsableLength int                `json:"min_usable_length,omitempty"`
	TimeBudgetMS    int                `json:"-"`
}

// ErrNoRoute is returned when a feature has no row in ai_feature_routes —
// a configuration error distinct from any provider failure, per the
// llm-gateway spec's "No routing config exists for a feature" scenario.
var ErrNoRoute = errors.New("llmgateway: no route configured for feature")

func (r *Repo) GetRoute(ctx context.Context, feature Feature) (Route, error) {
	var routeJSON []byte
	var timeBudgetMS int
	err := r.pool.QueryRow(ctx, `
		SELECT route, time_budget_ms FROM ai_feature_routes WHERE feature = $1
	`, string(feature)).Scan(&routeJSON, &timeBudgetMS)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Route{}, ErrNoRoute
		}
		return Route{}, fmt.Errorf("get route for feature %q: %w", feature, err)
	}
	var route Route
	if err := json.Unmarshal(routeJSON, &route); err != nil {
		return Route{}, fmt.Errorf("decode route for feature %q: %w", feature, err)
	}
	route.TimeBudgetMS = timeBudgetMS
	return route, nil
}

// LogUsage records one provider attempt (or a cache hit, when provider/
// model are empty and cacheHit is true) — the feature-tagged counterpart
// of internal/messaging.Repo.LogLLMUsage. Fire-and-forget: an attribution
// write must never affect whether a reply gets sent.
func (r *Repo) LogUsage(ctx context.Context, studioID uuid.UUID, feature Feature, provider, model string, latencyMs int, success bool, errMsg string, tokensIn, tokensOut int, costUSD float64, cacheHit bool) {
	var studioIDArg any
	if studioID != uuid.Nil {
		studioIDArg = studioID
	}
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO llm_usage_logs (studio_id, provider, model, latency_ms, success, error_msg, tokens_in, tokens_out, cost_usd, feature, cache_hit)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, studioIDArg, provider, model, latencyMs, success, errMsg, tokensIn, tokensOut, costUSD, string(feature), cacheHit)
}

// RecentCallsAllFailed mirrors internal/messaging.Repo.RecentLLMCallsAllFailed
// — see that method's doc comment for the "consecutive, nothing succeeding
// in between" semantics this implements identically against the same table.
func (r *Repo) RecentCallsAllFailed(ctx context.Context, provider string, n int) (bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT success FROM llm_usage_logs
		WHERE provider = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, provider, n)
	if err != nil {
		return false, fmt.Errorf("recent llm calls for %s: %w", provider, err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var success bool
		if err := rows.Scan(&success); err != nil {
			return false, fmt.Errorf("scan recent llm call: %w", err)
		}
		if success {
			return false, nil
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return count >= n, nil
}

// RecordAlert inserts a platform_alerts row — the llmgateway-side
// counterpart of internal/messaging.Repo.RecordAlert, writing the same
// table (see this package's doc comment).
func (r *Repo) RecordAlert(ctx context.Context, kind string, detail map[string]any, studioID uuid.UUID) {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		detailJSON = []byte(`{}`)
	}
	var studioIDArg any
	if studioID != uuid.Nil {
		studioIDArg = studioID
	}
	// Pass the JSON as a string, not a raw []byte: pgx can bind a []byte
	// parameter against a jsonb column in binary (bytea-shaped) form
	// depending on statement-cache/protocol state, which Postgres then
	// rejects with "invalid input syntax for type json" — a string
	// parameter is always sent as text, which jsonb's input function
	// always accepts. Found live (not caught by the unit/integration
	// tests, which use a fresh, uncontended pool per test run).
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO platform_alerts (kind, detail, studio_id)
		VALUES ($1, $2, $3)
	`, kind, string(detailJSON), studioIDArg)
}
