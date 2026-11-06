package studios

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestCheckAIAlertThresholds_ProviderFailureRateSpike covers task 3.8: a
// provider whose recent calls exceed the configured failure-rate
// threshold (with enough sample size) gets a platform_alerts row, and a
// second check within the same window does not duplicate it.
func TestCheckAIAlertThresholds_ProviderFailureRateSpike(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	// CheckAIAlertThresholds only ever checks the 4 real, known providers
	// (it loops platformAIProviderKeyNames), so a throwaway provider name
	// would never be examined — "openai" is used elsewhere as the safe
	// throwaway/unconfigured provider for tests, so its usage-log rows are
	// cleared and restored here too, to isolate this check from whatever
	// other tests logged for it.
	const provider = "openai"
	var anyStudioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&anyStudioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}

	if _, err := pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = $1`, provider); err != nil {
		t.Fatalf("clear existing openai usage logs: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = $1`, provider)
		_, _ = pool.Exec(ctx, `DELETE FROM platform_alerts WHERE kind = 'provider_failure_rate_spike' AND detail->>'provider' = $1`, provider)
	})

	// Save thresholds low enough that a small, deliberately-seeded sample
	// trips them: 5 calls, 4 failed = 80% > 50%.
	orig, err := repo.GetAIAlertThresholds(ctx)
	if err != nil {
		t.Fatalf("GetAIAlertThresholds: %v", err)
	}
	t.Cleanup(func() { _ = repo.SetAIAlertThresholds(context.Background(), orig, nil) })
	if err := repo.SetAIAlertThresholds(ctx, AIAlertThresholds{FailureRatePct: 50, FailureRateWindowMin: 15, DailySpendBudgetUSD: orig.DailySpendBudgetUSD}, nil); err != nil {
		t.Fatalf("SetAIAlertThresholds: %v", err)
	}

	for i := 0; i < 5; i++ {
		success := i == 0 // 1 success, 4 failures = 80% failure rate
		if _, err := pool.Exec(ctx, `
			INSERT INTO llm_usage_logs (studio_id, provider, model, latency_ms, success, error_msg, tokens_in, tokens_out, cost_usd, feature)
			VALUES ($1, $2, 'test-model', 10, $3, '', 0, 0, 0, 'test_feature')
		`, anyStudioID, provider, success); err != nil {
			t.Fatalf("seed usage log %d: %v", i, err)
		}
	}

	if err := repo.CheckAIAlertThresholds(ctx); err != nil {
		t.Fatalf("CheckAIAlertThresholds: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM platform_alerts WHERE kind = 'provider_failure_rate_spike' AND detail->>'provider' = $1
	`, provider).Scan(&count); err != nil {
		t.Fatalf("query alerts: %v", err)
	}
	if count != 1 {
		t.Fatalf("alert count after first check = %d, want 1", count)
	}

	// Second run within the same window must not duplicate.
	if err := repo.CheckAIAlertThresholds(ctx); err != nil {
		t.Fatalf("CheckAIAlertThresholds (second run): %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM platform_alerts WHERE kind = 'provider_failure_rate_spike' AND detail->>'provider' = $1
	`, provider).Scan(&count); err != nil {
		t.Fatalf("query alerts (second run): %v", err)
	}
	if count != 1 {
		t.Errorf("alert count after second check = %d, want still 1 (no duplicate)", count)
	}
}

// TestCheckAIAlertThresholds_BelowSampleSizeDoesNotAlert confirms a
// provider with too few recent calls never trips the failure-rate check,
// even at 100% failure — avoids false alarms on near-zero traffic.
func TestCheckAIAlertThresholds_BelowSampleSizeDoesNotAlert(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const provider = "openai" // must be a real, iterated provider — see the spike test's comment
	var anyStudioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&anyStudioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = $1`, provider); err != nil {
		t.Fatalf("clear existing openai usage logs: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = $1`, provider)
		_, _ = pool.Exec(ctx, `DELETE FROM platform_alerts WHERE kind = 'provider_failure_rate_spike' AND detail->>'provider' = $1`, provider)
	})

	if _, err := pool.Exec(ctx, `
		INSERT INTO llm_usage_logs (studio_id, provider, model, latency_ms, success, error_msg, tokens_in, tokens_out, cost_usd, feature)
		VALUES ($1, $2, 'test-model', 10, false, 'boom', 0, 0, 0, 'test_feature')
	`, anyStudioID, provider); err != nil {
		t.Fatalf("seed usage log: %v", err)
	}

	if err := repo.CheckAIAlertThresholds(ctx); err != nil {
		t.Fatalf("CheckAIAlertThresholds: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM platform_alerts WHERE kind = 'provider_failure_rate_spike' AND detail->>'provider' = $1
	`, provider).Scan(&count); err != nil {
		t.Fatalf("query alerts: %v", err)
	}
	if count != 0 {
		t.Errorf("alert count = %d, want 0 (sample size below minimum)", count)
	}
}

// TestCheckAIAlertThresholds_DailySpendOverBudget covers the spend-budget
// half of task 3.8.
func TestCheckAIAlertThresholds_DailySpendOverBudget(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	var anyStudioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&anyStudioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}
	const provider = "test_spend_budget_provider"
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM llm_usage_logs WHERE provider = $1`, provider)
		_, _ = pool.Exec(ctx, `DELETE FROM platform_alerts WHERE kind = 'daily_spend_over_budget'`)
	})

	orig, err := repo.GetAIAlertThresholds(ctx)
	if err != nil {
		t.Fatalf("GetAIAlertThresholds: %v", err)
	}
	t.Cleanup(func() { _ = repo.SetAIAlertThresholds(context.Background(), orig, nil) })
	if err := repo.SetAIAlertThresholds(ctx, AIAlertThresholds{FailureRatePct: orig.FailureRatePct, FailureRateWindowMin: orig.FailureRateWindowMin, DailySpendBudgetUSD: 1.0}, nil); err != nil {
		t.Fatalf("SetAIAlertThresholds: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO llm_usage_logs (studio_id, provider, model, latency_ms, success, error_msg, tokens_in, tokens_out, cost_usd, feature)
		VALUES ($1, $2, 'test-model', 10, true, '', 0, 0, 5.00, 'test_feature')
	`, anyStudioID, provider); err != nil {
		t.Fatalf("seed usage log: %v", err)
	}

	if err := repo.CheckAIAlertThresholds(ctx); err != nil {
		t.Fatalf("CheckAIAlertThresholds: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_alerts WHERE kind = 'daily_spend_over_budget'`).Scan(&count); err != nil {
		t.Fatalf("query alerts: %v", err)
	}
	if count != 1 {
		t.Fatalf("daily_spend_over_budget alert count = %d, want 1", count)
	}

	if err := repo.CheckAIAlertThresholds(ctx); err != nil {
		t.Fatalf("CheckAIAlertThresholds (second run): %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform_alerts WHERE kind = 'daily_spend_over_budget'`).Scan(&count); err != nil {
		t.Fatalf("query alerts (second run): %v", err)
	}
	if count != 1 {
		t.Errorf("daily_spend_over_budget alert count after second run = %d, want still 1 (no duplicate)", count)
	}
}
