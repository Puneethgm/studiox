package studios

import (
	"context"
	"testing"
)

// TestGetAIAlertThresholds_DefaultsWhenNeverSaved covers task 3.7: a
// fresh install (no platform_settings rows for these keys) must still
// return usable, documented defaults rather than zero values.
func TestGetAIAlertThresholds_DefaultsWhenNeverSaved(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	for _, key := range []string{aiAlertFailureRatePctSetting, aiAlertFailureRateWindowMinSetting, aiAlertDailySpendBudgetUSDSetting} {
		var orig string
		var had bool
		if err := pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, key).Scan(&orig); err == nil {
			had = true
		}
		key, orig, had := key, orig, had
		t.Cleanup(func() {
			ctx := context.Background()
			if had {
				_, _ = pool.Exec(ctx, `UPDATE platform_settings SET value = $2 WHERE key = $1`, key, orig)
			} else {
				_, _ = pool.Exec(ctx, `DELETE FROM platform_settings WHERE key = $1`, key)
			}
		})
		if _, err := pool.Exec(ctx, `DELETE FROM platform_settings WHERE key = $1`, key); err != nil {
			t.Fatalf("clear %s: %v", key, err)
		}
	}

	got, err := repo.GetAIAlertThresholds(ctx)
	if err != nil {
		t.Fatalf("GetAIAlertThresholds: %v", err)
	}
	if got.FailureRatePct != defaultAIAlertFailureRatePct {
		t.Errorf("FailureRatePct = %v, want default %v", got.FailureRatePct, defaultAIAlertFailureRatePct)
	}
	if got.FailureRateWindowMin != defaultAIAlertFailureRateWindowMin {
		t.Errorf("FailureRateWindowMin = %v, want default %v", got.FailureRateWindowMin, defaultAIAlertFailureRateWindowMin)
	}
	if got.DailySpendBudgetUSD != defaultAIAlertDailySpendBudgetUSD {
		t.Errorf("DailySpendBudgetUSD = %v, want default %v", got.DailySpendBudgetUSD, defaultAIAlertDailySpendBudgetUSD)
	}
}

// TestSetGetAIAlertThresholds_RoundTrips covers the save+read path and
// that a save persists all three fields together.
func TestSetGetAIAlertThresholds_RoundTrips(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	for _, key := range []string{aiAlertFailureRatePctSetting, aiAlertFailureRateWindowMinSetting, aiAlertDailySpendBudgetUSDSetting} {
		var orig string
		var had bool
		if err := pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, key).Scan(&orig); err == nil {
			had = true
		}
		key, orig, had := key, orig, had
		t.Cleanup(func() {
			ctx := context.Background()
			if had {
				_, _ = pool.Exec(ctx, `UPDATE platform_settings SET value = $2 WHERE key = $1`, key, orig)
			} else {
				_, _ = pool.Exec(ctx, `DELETE FROM platform_settings WHERE key = $1`, key)
			}
		})
	}

	want := AIAlertThresholds{FailureRatePct: 33.5, FailureRateWindowMin: 20, DailySpendBudgetUSD: 123.45}
	if err := repo.SetAIAlertThresholds(ctx, want, nil); err != nil {
		t.Fatalf("SetAIAlertThresholds: %v", err)
	}

	got, err := repo.GetAIAlertThresholds(ctx)
	if err != nil {
		t.Fatalf("GetAIAlertThresholds: %v", err)
	}
	if got != want {
		t.Errorf("GetAIAlertThresholds = %+v, want %+v", got, want)
	}
}

func TestSetAIAlertThresholds_Validation(t *testing.T) {
	repo, _ := setupAIKeyStoreTest(t)
	ctx := context.Background()

	cases := []struct {
		name string
		t    AIAlertThresholds
	}{
		{"negative failure rate", AIAlertThresholds{FailureRatePct: -1, FailureRateWindowMin: 15, DailySpendBudgetUSD: 10}},
		{"failure rate over 100", AIAlertThresholds{FailureRatePct: 101, FailureRateWindowMin: 15, DailySpendBudgetUSD: 10}},
		{"zero window", AIAlertThresholds{FailureRatePct: 50, FailureRateWindowMin: 0, DailySpendBudgetUSD: 10}},
		{"negative window", AIAlertThresholds{FailureRatePct: 50, FailureRateWindowMin: -5, DailySpendBudgetUSD: 10}},
		{"negative budget", AIAlertThresholds{FailureRatePct: 50, FailureRateWindowMin: 15, DailySpendBudgetUSD: -1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := repo.SetAIAlertThresholds(ctx, c.t, nil); err == nil {
				t.Errorf("SetAIAlertThresholds(%+v) = nil error, want a validation error", c.t)
			}
		})
	}
}

func TestListRecentPlatformAlerts(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	const kind = "test_list_recent_alerts"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM platform_alerts WHERE kind = $1`, kind)
	})

	if _, err := pool.Exec(ctx, `
		INSERT INTO platform_alerts (kind, detail) VALUES ($1, '{"note":"first"}'::jsonb), ($1, '{"note":"second"}'::jsonb)
	`, kind); err != nil {
		t.Fatalf("seed alerts: %v", err)
	}

	alerts, err := repo.ListRecentPlatformAlerts(ctx, 50)
	if err != nil {
		t.Fatalf("ListRecentPlatformAlerts: %v", err)
	}
	found := 0
	for _, a := range alerts {
		if a.Kind == kind {
			found++
			if a.Detail["note"] != "first" && a.Detail["note"] != "second" {
				t.Errorf("unexpected detail for alert %s: %+v", a.ID, a.Detail)
			}
		}
	}
	if found != 2 {
		t.Errorf("found %d alerts with kind %q, want 2", found, kind)
	}
}
