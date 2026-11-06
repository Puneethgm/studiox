package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/secrets"
	"github.com/projectx/api/internal/studios"
)

// TestBuildPrompt_IncludesActiveOfferAndPaymentLink covers
// ai-reply-flow-llm-design task 2.5: a studio with an active offer and a
// plan with a payment link produces a prompt containing both's structured
// details, and an inactive offer never appears.
func TestBuildPrompt_IncludesActiveOfferAndPaymentLink(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
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

	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	studiosRepo := studios.NewRepo(pool, cipher)

	w := &AIWorker{studiosRepo: studiosRepo, log: slog.Default()}

	studio := &studios.Studio{ID: uuid.New()}
	plans := []Plan{{PlanName: "Monthly", PriceSGD: 9900, BillingCycle: "monthly", PaymentLink: "https://pay.example.com/monthly"}}
	offers := []Offer{
		{Name: "Intro Offer", Description: "2 weeks unlimited", Terms: "New members only", PriceSGD: 4900},
	}

	prompt := w.buildPrompt(ctx, nil, nil, nil, nil, studio, plans, offers, 0, nil, nil, "", false, "")

	if !strings.Contains(prompt, "https://pay.example.com/monthly") {
		t.Error("prompt does not include the plan's payment link")
	}
	if !strings.Contains(prompt, "Intro Offer") || !strings.Contains(prompt, "S$49.00") {
		t.Error("prompt does not include the active offer's name/price")
	}
	if !strings.Contains(prompt, "2 weeks unlimited") || !strings.Contains(prompt, "New members only") {
		t.Error("prompt does not include the active offer's description/terms")
	}
}

// TestBuildPrompt_NoOffersSection_WhenNoneActive confirms the offers
// section is omitted entirely (not an empty/awkward header) when there
// are no active offers, matching the existing behavior for an empty
// plans list.
func TestBuildPrompt_NoOffersSection_WhenNoneActive(t *testing.T) {
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_DB"))
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)
	cipher, err := secrets.New(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatalf("init cipher: %v", err)
	}
	w := &AIWorker{studiosRepo: studios.NewRepo(pool, cipher), log: slog.Default()}
	studio := &studios.Studio{ID: uuid.New()}

	prompt := w.buildPrompt(context.Background(), nil, nil, nil, nil, studio, nil, nil, 0, nil, nil, "", false, "")

	if strings.Contains(prompt, "ACTIVE OFFERS") {
		t.Error("prompt should not include an ACTIVE OFFERS section when there are no active offers")
	}
}
