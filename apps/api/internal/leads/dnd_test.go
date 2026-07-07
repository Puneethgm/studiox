package leads

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// testDB connects using the same .env-driven DSN pattern as the messaging
// package's integration tests, skipping when no DB env vars are configured.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Skip("Skipping integration test; no DB env vars found")
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedTestLead inserts a minimal lead directly (bypassing CreateLeadWithOutbox
// so no outbox/autocontact side effects fire), scoped to an existing
// studio+campaign found in the DB, and registers cleanup.
func seedTestLead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) (studioID, leadID uuid.UUID) {
	t.Helper()
	var campaignID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id, studio_id FROM campaigns LIMIT 1`).Scan(&campaignID, &studioID)
	if err != nil {
		t.Skip("Skipping test; no campaign found in DB to attach test lead to")
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan, source, status)
		VALUES ($1, $2, 'DND Test Lead', 'DND', 'Test', $3, '9999999999', 'General', 'test', 'new')
		RETURNING id
	`, studioID, campaignID, email).Scan(&leadID)
	if err != nil {
		t.Fatalf("seed test lead: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID)
	})
	return studioID, leadID
}

func TestSetDNDEnabled(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)

	studioID, leadID := seedTestLead(t, ctx, pool, "dnd-repo-test@example.com")

	// New leads default to DND disabled.
	lead, err := repo.GetLead(ctx, studioID, leadID)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.DNDEnabled {
		t.Fatalf("new lead DNDEnabled = true; want false")
	}

	// Enable DND.
	if err := repo.SetDNDEnabled(ctx, studioID, leadID, true); err != nil {
		t.Fatalf("SetDNDEnabled(true): %v", err)
	}
	lead, err = repo.GetLead(ctx, studioID, leadID)
	if err != nil {
		t.Fatalf("GetLead after enable: %v", err)
	}
	if !lead.DNDEnabled {
		t.Errorf("DNDEnabled after enable = false; want true")
	}
	// Status must be untouched by DND toggling.
	if lead.Status != StatusNew {
		t.Errorf("Status after DND enable = %q; want %q (DND must not change pipeline status)", lead.Status, StatusNew)
	}

	// Disable DND.
	if err := repo.SetDNDEnabled(ctx, studioID, leadID, false); err != nil {
		t.Fatalf("SetDNDEnabled(false): %v", err)
	}
	lead, err = repo.GetLead(ctx, studioID, leadID)
	if err != nil {
		t.Fatalf("GetLead after disable: %v", err)
	}
	if lead.DNDEnabled {
		t.Errorf("DNDEnabled after disable = true; want false")
	}

	// Unknown lead ID.
	if err := repo.SetDNDEnabled(ctx, studioID, uuid.New(), true); err != ErrLeadNotFound {
		t.Errorf("SetDNDEnabled for unknown lead = %v; want ErrLeadNotFound", err)
	}
}

func TestService_SetDND_CancelsPendingMessages(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	repo := NewRepo(pool)
	svc := NewService(repo, nil)

	studioID, leadID := seedTestLead(t, ctx, pool, "dnd-service-test@example.com")

	var cancelCalled bool
	var cancelledStudioID, cancelledLeadID uuid.UUID
	svc.SetCancelPendingMessagesFunc(func(ctx context.Context, sID, lID uuid.UUID) (int, error) {
		cancelCalled = true
		cancelledStudioID = sID
		cancelledLeadID = lID
		return 3, nil
	})

	lead, err := svc.SetDND(ctx, studioID, leadID, true)
	if err != nil {
		t.Fatalf("SetDND(true): %v", err)
	}
	if !lead.DNDEnabled {
		t.Errorf("returned lead DNDEnabled = false; want true")
	}
	if !cancelCalled {
		t.Errorf("cancelPendingMessages callback was not invoked when DND turned on")
	}
	if cancelledStudioID != studioID || cancelledLeadID != leadID {
		t.Errorf("cancelPendingMessages called with (%s, %s); want (%s, %s)",
			cancelledStudioID, cancelledLeadID, studioID, leadID)
	}

	// Turning DND off must NOT invoke the cancellation callback again.
	cancelCalled = false
	if _, err := svc.SetDND(ctx, studioID, leadID, false); err != nil {
		t.Fatalf("SetDND(false): %v", err)
	}
	if cancelCalled {
		t.Errorf("cancelPendingMessages callback was invoked when DND turned OFF; should only fire on enable")
	}
}
