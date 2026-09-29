package studios

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
)

// TestResolveTrialAmountSGD_IgnoresCheapMembershipPlan is a regression test
// for a real bug: the fallback (used when a studio hasn't set an explicit
// trial amount) used to pick whichever ACTIVE plan was cheapest, regardless
// of name — so a discounted or entry-level monthly membership priced below
// the studio's real trial price got mistaken for the trial. The fix
// restricts the fallback to the plan literally named "Trial" — the same
// plan_name == "Trial" convention messaging.ListActivePlans already uses
// to exclude it from the membership list.
func TestResolveTrialAmountSGD_IgnoresCheapMembershipPlan(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
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
	defer pool.Close()

	var studioID uuid.UUID
	var realTrialPriceSGD int
	// Needs a studio with an actual active "Trial" plan already seeded —
	// don't create a second one (plan_name isn't unique, and a $0-priced
	// plan interacts oddly with the fallback's "amount == 0 means unset"
	// check, which is a separate, pre-existing quirk out of scope here).
	err = pool.QueryRow(ctx, `
		SELECT p.studio_id, p.price_sgd
		FROM plans p
		WHERE p.plan_name = 'Trial' AND p.is_active = true AND p.price_sgd > 0
		LIMIT 1
	`).Scan(&studioID, &realTrialPriceSGD)
	if err != nil {
		t.Skip("Skipping test; no studio with an active, non-zero-priced Trial plan found in DB")
	}

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)
	svc := NewService(repo, nil, nil, nil, nil, nil, "")

	// A cheap, active MONTHLY membership plan, priced below the real Trial
	// plan — this must never be mistaken for the trial price.
	cheapMembership, err := repo.CreatePlan(ctx, studioID, CreatePlanInput{
		PlanName: "Regression Test Cheap Membership", PriceSGD: 1, BillingCycle: "monthly", IsActive: true,
	})
	if err != nil {
		t.Fatalf("create cheap membership plan: %v", err)
	}
	t.Cleanup(func() { _ = repo.DeletePlan(context.Background(), studioID, cheapMembership.ID) })

	// trialAmountSGD=0 forces the fallback path.
	got := svc.ResolveTrialAmountSGD(ctx, studioID, 0)
	if got != int64(realTrialPriceSGD) {
		t.Errorf("ResolveTrialAmountSGD = %d, want %d (the real Trial plan) — got the cheap membership's price instead if this is 1", got, realTrialPriceSGD)
	}
}
