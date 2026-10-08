package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// setupBroadcastProgressTest builds a throwaway studio, broadcast list and
// campaign with two recipients already enqueued — one whose outbound job has
// actually gone out ('sent'), one whose job is still in flight ('pending',
// e.g. waiting its turn behind the daily send limit, or mid-retry-backoff).
func setupBroadcastProgressTest(t *testing.T) (repo *Repo, pool *pgxpool.Pool, studioID, campaignID, convID uuid.UUID, inFlightJobID int64) {
	t.Helper()
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

	studioID = uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color, contact_email) VALUES ($1,$2,'Broadcast Progress Test','#7c3aed','owner@example.com')`,
		studioID, "broadcast-progress-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	channelID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO channel_accounts (id, studio_id, kind, external_id, display_handle, access_token_enc) VALUES ($1,$2,'whatsapp_web','test-channel','Test','enc')`,
		channelID, studioID); err != nil {
		t.Fatalf("create test channel: %v", err)
	}

	listID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_lists (id, studio_id, name) VALUES ($1,$2,'Test List')`, listID, studioID); err != nil {
		t.Fatalf("create test list: %v", err)
	}

	campaignID = uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_campaigns (id, studio_id, broadcast_list_id, body, scheduled_for, status, total_count)
		VALUES ($1,$2,$3,'hello','2026-01-01','sending',2)`, campaignID, studioID, listID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}

	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone','6591234567','Test Contact')`,
		contactID, studioID); err != nil {
		t.Fatalf("create test contact identity: %v", err)
	}
	conv := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,'test-thread',true)`,
		conv, studioID, channelID, contactID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	convID = conv

	// Recipient 1: already sent.
	sentJobID := insertOutboundJob(t, ctx, pool, studioID, convID, "sent")
	bc1 := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_contacts (id, broadcast_list_id, name, phone) VALUES ($1,$2,'Sent Contact','+6591111111')`, bc1, listID); err != nil {
		t.Fatalf("create contact 1: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_campaign_recipients (campaign_id, broadcast_contact_id, status, outbound_job_id, enqueued_at)
		VALUES ($1,$2,'enqueued',$3,now())`, campaignID, bc1, sentJobID); err != nil {
		t.Fatalf("create recipient 1: %v", err)
	}

	// Recipient 2: enqueued but its job hasn't actually gone out yet.
	inFlightJobID = insertOutboundJob(t, ctx, pool, studioID, convID, "pending")
	bc2 := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_contacts (id, broadcast_list_id, name, phone) VALUES ($1,$2,'In Flight Contact','+6592222222')`, bc2, listID); err != nil {
		t.Fatalf("create contact 2: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO broadcast_campaign_recipients (campaign_id, broadcast_contact_id, status, outbound_job_id, enqueued_at)
		VALUES ($1,$2,'enqueued',$3,now())`, campaignID, bc2, inFlightJobID); err != nil {
		t.Fatalf("create recipient 2: %v", err)
	}

	repo = NewRepo(pool, nil)
	return repo, pool, studioID, campaignID, convID, inFlightJobID
}

func insertOutboundJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, studioID, convID uuid.UUID, status string) int64 {
	t.Helper()
	var jobID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO outbound_jobs (studio_id, conversation_id, body, source_kind, status)
		VALUES ($1,$2,'hello','automation',$3)
		RETURNING id
	`, studioID, convID, status).Scan(&jobID); err != nil {
		t.Fatalf("create outbound job: %v", err)
	}
	return jobID
}

// The real-world bug this guards against: a broadcast to 100+ contacts with a
// daily send limit of 48 queues the last batch, which immediately shows
// "Completed" even though those last messages are still sitting unsent in the
// outbound queue — misleading staff into thinking the whole thing already went
// out. A campaign must stay in 'sending' until every recipient's job has
// actually reached a terminal state (sent or dead), not merely been enqueued.
func TestAdvanceBroadcastCampaignProgress_StaysSendingWhileAJobIsStillInFlight(t *testing.T) {
	repo, pool, _, campaignID, _, inFlightJobID := setupBroadcastProgressTest(t)
	ctx := context.Background()

	if err := repo.AdvanceBroadcastCampaignProgress(ctx, campaignID); err != nil {
		t.Fatalf("AdvanceBroadcastCampaignProgress: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM broadcast_campaigns WHERE id = $1`, campaignID).Scan(&status); err != nil {
		t.Fatalf("read campaign: %v", err)
	}
	if status != "sending" {
		t.Errorf("campaign status = %q, want %q — one recipient's job is still pending delivery", status, "sending")
	}

	// Now the in-flight job actually goes out...
	if _, err := pool.Exec(ctx, `UPDATE outbound_jobs SET status = 'sent', sent_at = now() WHERE id = $1`, inFlightJobID); err != nil {
		t.Fatalf("mark job sent: %v", err)
	}
	if err := repo.AdvanceBroadcastCampaignProgress(ctx, campaignID); err != nil {
		t.Fatalf("AdvanceBroadcastCampaignProgress (2nd call): %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM broadcast_campaigns WHERE id = $1`, campaignID).Scan(&status); err != nil {
		t.Fatalf("read campaign: %v", err)
	}
	if status != "completed" {
		t.Errorf("campaign status = %q, want %q — every recipient's job has now actually been sent", status, "completed")
	}
}
