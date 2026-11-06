package studios

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestOffersRepo_CRUD covers ai-reply-flow-llm-design task 2.3: create,
// list (all + active-only), update, and delete against the real local
// DB.
func TestOffersRepo_CRUD(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}

	offer, err := repo.CreateOffer(ctx, studioID, CreateOfferInput{
		Name:        "Eval Test Offer",
		Description: "2 weeks unlimited",
		Terms:       "New members only",
		PriceSGD:    4900,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	t.Cleanup(func() { _ = repo.DeleteOffer(context.Background(), studioID, offer.ID) })

	if offer.Name != "Eval Test Offer" || offer.PriceSGD != 4900 || !offer.IsActive {
		t.Errorf("CreateOffer returned unexpected offer: %+v", offer)
	}

	all, err := repo.ListOffers(ctx, studioID)
	if err != nil {
		t.Fatalf("ListOffers: %v", err)
	}
	found := false
	for _, o := range all {
		if o.ID == offer.ID {
			found = true
		}
	}
	if !found {
		t.Error("ListOffers did not include the newly created offer")
	}

	active, err := repo.ListActiveOffers(ctx, studioID)
	if err != nil {
		t.Fatalf("ListActiveOffers: %v", err)
	}
	foundActive := false
	for _, o := range active {
		if o.ID == offer.ID {
			foundActive = true
		}
	}
	if !foundActive {
		t.Error("ListActiveOffers did not include the active offer")
	}

	newDesc := "Updated description"
	isActive := false
	if err := repo.UpdateOffer(ctx, studioID, offer.ID, UpdateOfferInput{
		Description: &newDesc,
		IsActive:    &isActive,
	}); err != nil {
		t.Fatalf("UpdateOffer: %v", err)
	}

	activeAfter, err := repo.ListActiveOffers(ctx, studioID)
	if err != nil {
		t.Fatalf("ListActiveOffers after deactivate: %v", err)
	}
	for _, o := range activeAfter {
		if o.ID == offer.ID {
			t.Error("deactivated offer should not appear in ListActiveOffers")
		}
	}

	if err := repo.DeleteOffer(ctx, studioID, offer.ID); err != nil {
		t.Fatalf("DeleteOffer: %v", err)
	}
	if err := repo.DeleteOffer(ctx, studioID, offer.ID); err != ErrNotFound {
		t.Errorf("DeleteOffer on already-deleted offer = %v, want ErrNotFound", err)
	}
}

// TestPlansRepo_PaymentLink covers the payment_link field added by the
// same migration (task 2.2): it round-trips through create, list, and
// partial update.
func TestPlansRepo_PaymentLink(t *testing.T) {
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()

	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}

	plan, err := repo.CreatePlan(ctx, studioID, CreatePlanInput{
		PlanName:    "Eval Test Plan",
		PriceSGD:    9900,
		IsActive:    true,
		PaymentLink: "https://pay.example.com/eval-test-plan",
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	t.Cleanup(func() { _ = repo.DeletePlan(context.Background(), studioID, plan.ID) })

	if plan.PaymentLink != "https://pay.example.com/eval-test-plan" {
		t.Errorf("CreatePlan: PaymentLink = %q, want the configured link", plan.PaymentLink)
	}

	all, err := repo.ListPlans(ctx, studioID)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	found := false
	for _, p := range all {
		if p.ID == plan.ID {
			found = true
			if p.PaymentLink != plan.PaymentLink {
				t.Errorf("ListPlans: PaymentLink = %q, want %q", p.PaymentLink, plan.PaymentLink)
			}
		}
	}
	if !found {
		t.Fatal("ListPlans did not include the newly created plan")
	}

	newLink := "https://pay.example.com/updated"
	if err := repo.UpdatePlan(ctx, studioID, plan.ID, UpdatePlanInput{PaymentLink: &newLink}); err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	after, err := repo.ListPlans(ctx, studioID)
	if err != nil {
		t.Fatalf("ListPlans after update: %v", err)
	}
	for _, p := range after {
		if p.ID == plan.ID && p.PaymentLink != newLink {
			t.Errorf("after UpdatePlan: PaymentLink = %q, want %q", p.PaymentLink, newLink)
		}
	}
}
