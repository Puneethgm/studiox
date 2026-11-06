package llmgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

// fakeCompleter lets tests control exactly what each successive Bifrost
// call returns, without any live network call — see bifrostCompleter's
// doc comment for why Client depends on this interface instead of the
// concrete *bifrost.Client.
type fakeCompleter struct {
	mu    sync.Mutex
	calls []bifrost.Request
	// responses are consumed in order, one per call; the last entry repeats
	// if more calls happen than responses provided.
	responses []fakeResponse
}

type fakeResponse struct {
	reply bifrost.Reply
	err   error
	delay time.Duration // if set, blocks until ctx.Done() or this elapses
}

func (f *fakeCompleter) Complete(ctx context.Context, req bifrost.Request) (bifrost.Reply, error) {
	f.mu.Lock()
	idx := len(f.calls)
	f.calls = append(f.calls, req)
	var resp fakeResponse
	if idx < len(f.responses) {
		resp = f.responses[idx]
	} else if len(f.responses) > 0 {
		resp = f.responses[len(f.responses)-1]
	}
	f.mu.Unlock()

	if resp.delay > 0 {
		select {
		case <-ctx.Done():
			return bifrost.Reply{}, ctx.Err()
		case <-time.After(resp.delay):
		}
	}
	return resp.reply, resp.err
}

func (f *fakeCompleter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// setupGatewayTest returns a Client wired to the real test DB for
// llmRepo/studiosRepo/repo (per this project's "real Postgres, no mocks
// for DB" convention — see docs/skills.md) but with a fake, in-memory
// bifrostCompleter standing in for the actual provider network call.
// providerKeys seeds a throwaway platform_ai_keys row for each listed
// provider (groq/gemini/claude/openai — the only provider names this
// package understands) so GetPlatformAIProviderKey returns non-empty for
// them, restoring any prior state on cleanup.
// setupGatewayTest's returned uuid.UUID is a real existing studios.id —
// llm_usage_logs.studio_id has an FK to studios(id), so using
// uuid.New() (a random, non-existent id) in a test would make every
// LogUsage insert silently fail the FK check (LogUsage swallows insert
// errors, same as the production code it mirrors), leaving the table
// looking empty no matter how many Complete() calls ran.
func setupGatewayTest(t *testing.T, completer *fakeCompleter, providerKeys ...string) (*Client, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	_ = godotenv.Load("../../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	studiosRepo := studios.NewRepo(pool, cipher)
	llmRepo := llm.NewRepo(pool, cipher)
	gwRepo := NewRepo(pool)

	for _, provider := range providerKeys {
		var hadRow bool
		var origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus string
		if err := pool.QueryRow(ctx, `SELECT key_ciphertext, data_key_ciphertext, key_fingerprint, status FROM platform_ai_keys WHERE provider = $1`, provider).
			Scan(&origKeyCiphertext, &origDataKeyCiphertext, &origLastFour, &origStatus); err == nil {
			hadRow = true
		}
		provider := provider
		t.Cleanup(func() {
			ctx := context.Background()
			if hadRow {
				_, _ = pool.Exec(ctx, `
					UPDATE platform_ai_keys SET key_ciphertext=$2, data_key_ciphertext=$3, key_fingerprint=$4, status=$5 WHERE provider=$1
				`, provider, origKeyCiphertext, origDataKeyCiphertext, origLastFour, origStatus)
			} else {
				_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_keys WHERE provider = $1`, provider)
			}
			_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = $1`, provider)
		})
		if err := studiosRepo.SetPlatformAIProviderKey(ctx, provider, "test-key-"+provider, nil); err != nil {
			t.Fatalf("seed key for %s: %v", provider, err)
		}
	}

	client := &Client{
		bifrostClient: completer,
		llmRepo:       llmRepo,
		studiosRepo:   studiosRepo,
		repo:          gwRepo,
		log:           slog.Default(),
		routeCache:    make(map[Feature]cachedRoute),
	}
	return client, pool, studioID
}

// seedRoute inserts (or replaces) an ai_feature_routes row for a throwaway
// test feature, cleaning it up afterward.
func seedRoute(t *testing.T, pool *pgxpool.Pool, feature Feature, route Route, timeBudgetMS int) {
	t.Helper()
	ctx := context.Background()
	routeJSON := fmt.Sprintf(`{"providers": %s, "max_tokens": {}, "temperature": {}, "min_usable_length": %d}`,
		jsonStringArray(route.Providers), route.MinUsableLength)
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_feature_routes (feature, route, time_budget_ms) VALUES ($1, $2::jsonb, $3)
		ON CONFLICT (feature) DO UPDATE SET route = EXCLUDED.route, time_budget_ms = EXCLUDED.time_budget_ms
	`, string(feature), routeJSON, timeBudgetMS); err != nil {
		t.Fatalf("seed route for %s: %v", feature, err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM ai_feature_routes WHERE feature = $1`, string(feature))
		_, _ = pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE feature = $1`, string(feature))
	})
}

func jsonStringArray(ss []string) string {
	out := "["
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += `"` + s + `"`
	}
	return out + "]"
}

func TestComplete_PrimarySuccess(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: "hello from openai"}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_primary_success")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)

	text, ref, err := client.Complete(context.Background(), feature, studioID, "hi")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text != "hello from openai" {
		t.Errorf("text = %q, want %q", text, "hello from openai")
	}
	if ref == "" {
		t.Error("expected a non-empty sourceRef on success")
	}
	if completer.callCount() != 1 {
		t.Errorf("expected exactly 1 call for a primary success, got %d", completer.callCount())
	}
}

func TestComplete_FallbackOnError(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{err: errors.New("bifrost: openai/gpt-4o-mini status 500: internal error")},
		{reply: bifrost.Reply{Text: "hello from gemini"}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai", "gemini")
	feature := Feature("test_fallback_on_error")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai", "gemini"}}, 60000)

	text, ref, err := client.Complete(context.Background(), feature, studioID, "hi")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text != "hello from gemini" {
		t.Errorf("text = %q, want %q", text, "hello from gemini")
	}
	if ref == "" || ref[:6] != "gemini" {
		t.Errorf("sourceRef = %q, want it to start with \"gemini\"", ref)
	}
}

func TestComplete_FallbackOnEmptyResponse(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{reply: bifrost.Reply{Text: ""}}, // no error, but unusable
		{reply: bifrost.Reply{Text: "hello from gemini"}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai", "gemini")
	feature := Feature("test_fallback_on_empty")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai", "gemini"}}, 60000)

	text, ref, err := client.Complete(context.Background(), feature, studioID, "hi")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text != "hello from gemini" {
		t.Errorf("text = %q, want %q", text, "hello from gemini")
	}
	if ref == "" {
		t.Error("expected a non-empty sourceRef")
	}
}

func TestComplete_AllProvidersFail_ReturnsEmptyNotError(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{err: errors.New("boom")},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_all_fail")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)

	text, ref, err := client.Complete(context.Background(), feature, studioID, "hi")
	if err != nil {
		t.Fatalf("Complete should not return a hard error when every provider fails, got: %v", err)
	}
	if text != "" || ref != "" {
		t.Errorf("Complete() = (%q, %q), want empty strings when every provider fails", text, ref)
	}
}

func TestComplete_TimeBudgetExceeded(t *testing.T) {
	completer := &fakeCompleter{responses: []fakeResponse{
		{delay: 2 * time.Second, reply: bifrost.Reply{Text: "too slow"}},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_time_budget")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 50) // 50ms budget

	start := time.Now()
	text, ref, err := client.Complete(context.Background(), feature, studioID, "hi")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text != "" || ref != "" {
		t.Errorf("Complete() = (%q, %q), want empty when the provider exceeds the time budget", text, ref)
	}
	if elapsed > 1*time.Second {
		t.Errorf("Complete took %v, want it to stop at roughly the 50ms budget, not wait out the fake's 2s delay", elapsed)
	}
}

func TestComplete_NoRouteConfigured(t *testing.T) {
	completer := &fakeCompleter{}
	client, _, studioID := setupGatewayTest(t, completer)

	_, _, err := client.Complete(context.Background(), Feature("test_feature_with_no_route_ever_seeded"), studioID, "hi")
	if !errors.Is(err, ErrNoRoute) {
		t.Errorf("Complete err = %v, want ErrNoRoute", err)
	}
}

func TestGetRoute_CachesWithinTTL(t *testing.T) {
	completer := &fakeCompleter{}
	client, pool, _ := setupGatewayTest(t, completer)
	feature := Feature("test_route_caching")
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)

	route1, err := client.getRoute(context.Background(), feature)
	if err != nil {
		t.Fatalf("getRoute (first): %v", err)
	}
	if len(route1.Providers) != 1 || route1.Providers[0] != "openai" {
		t.Fatalf("unexpected initial route: %+v", route1)
	}

	// Mutate the DB directly — a cached read must not see this yet.
	if _, err := pool.Exec(context.Background(), `
		UPDATE ai_feature_routes SET route = '{"providers": ["claude"]}'::jsonb WHERE feature = $1
	`, string(feature)); err != nil {
		t.Fatalf("mutate route: %v", err)
	}

	route2, err := client.getRoute(context.Background(), feature)
	if err != nil {
		t.Fatalf("getRoute (second, should be cached): %v", err)
	}
	if len(route2.Providers) != 1 || route2.Providers[0] != "openai" {
		t.Errorf("getRoute returned the mutated DB value instead of the cached one: %+v", route2)
	}
}

func TestIsUsable(t *testing.T) {
	cases := []struct {
		text      string
		minLength int
		want      bool
	}{
		{"hello", 0, true},
		{"", 0, false},
		{"   ", 0, false},
		{"hi", 15, false},
		{"this is long enough", 15, true},
	}
	for _, c := range cases {
		if got := isUsable(c.text, c.minLength); got != c.want {
			t.Errorf("isUsable(%q, %d) = %v, want %v", c.text, c.minLength, got, c.want)
		}
	}
}

func TestComplete_AuthFailureFlagsKeyAsFailing(t *testing.T) {
	authErr := errors.New("bifrost: openai/gpt-4o-mini status 401: Incorrect API key provided")
	completer := &fakeCompleter{responses: []fakeResponse{
		{err: authErr}, {err: authErr}, {err: authErr},
	}}
	client, pool, studioID := setupGatewayTest(t, completer, "openai")
	feature := Feature("test_auth_failure_flagging")
	// A single-model route so each Complete call makes exactly one attempt.
	if _, err := pool.Exec(context.Background(), `
		UPDATE platform_ai_models SET enabled = false WHERE provider = 'openai'
	`); err != nil {
		t.Fatalf("disable any pre-existing openai models: %v", err)
	}
	seedRoute(t, pool, feature, Route{Providers: []string{"openai"}}, 60000)
	// Isolate from any openai rows other tests in this package logged —
	// RecentCallsAllFailed looks at the last N rows for the provider across
	// the whole table, not scoped to this test's feature.
	if _, err := pool.Exec(context.Background(), `DELETE FROM llm_usage_logs WHERE provider = 'openai'`); err != nil {
		t.Fatalf("clear prior openai usage logs: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = 'openai' AND feature = $1`, string(feature))
		_, _ = pool.Exec(ctx, `DELETE FROM platform_ai_key_audit WHERE provider = 'openai' AND action = 'flagged_failing'`)
		_, _ = pool.Exec(ctx, `UPDATE platform_ai_keys SET status = 'active' WHERE provider = 'openai'`)
	})

	for i := 0; i < failingKeyConsecutiveThreshold; i++ {
		_, _, err := client.Complete(context.Background(), feature, studioID, "hi")
		if err != nil {
			t.Fatalf("Complete (call %d): %v", i, err)
		}
	}

	status, err := client.studiosRepo.GetPlatformAIProviderKeyStatus(context.Background(), "openai")
	if err != nil {
		t.Fatalf("GetPlatformAIProviderKeyStatus: %v", err)
	}
	if status != studios.AIKeyStatusFailing {
		t.Errorf("openai key status = %q, want %q after %d consecutive auth failures", status, studios.AIKeyStatusFailing, failingKeyConsecutiveThreshold)
	}
}

// TestRecordAlert_ActuallyPersistsToPlatformAlerts is a regression test
// for a real bug found via live testing: passing json.Marshal's raw
// []byte for the jsonb `detail` column intermittently made pgx send it in
// a form Postgres rejected ("invalid input syntax for type json"), and
// since RecordAlert is fire-and-forget (errors are intentionally
// swallowed — an attribution/alerting failure must never block an AI
// reply), the row silently never appeared. This asserts the actual
// end state (a readable row), not just "no error returned", so a
// regression here fails loudly instead of being swallowed again.
func TestRecordAlert_ActuallyPersistsToPlatformAlerts(t *testing.T) {
	_, pool, _ := setupGatewayTest(t, &fakeCompleter{})
	repo := NewRepo(pool)

	const kind = "test_record_alert_regression"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_alerts WHERE kind = $1`, kind)
	})

	repo.RecordAlert(context.Background(), kind, map[string]any{"provider": "openai", "n": 3}, uuid.Nil)

	var detailJSON []byte
	var studioID *uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		SELECT detail, studio_id FROM platform_alerts WHERE kind = $1
	`, kind).Scan(&detailJSON, &studioID); err != nil {
		t.Fatalf("RecordAlert did not persist a readable row: %v", err)
	}
	if studioID != nil {
		t.Errorf("studio_id = %v, want NULL for a platform-wide alert (uuid.Nil)", *studioID)
	}
	var detail map[string]any
	if err := json.Unmarshal(detailJSON, &detail); err != nil {
		t.Fatalf("stored detail is not valid JSON: %v", err)
	}
	if detail["provider"] != "openai" {
		t.Errorf("detail[provider] = %v, want %q", detail["provider"], "openai")
	}
}
