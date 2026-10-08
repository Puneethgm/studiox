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

// setupManualReplyCancelTest builds a throwaway studio, user, and
// conversation to send a manual reply against.
func setupManualReplyCancelTest(t *testing.T) (svc *Service, pool *pgxpool.Pool, studioID, convID, userID uuid.UUID) {
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
	if _, err := pool.Exec(ctx, `INSERT INTO studios (id, slug, name, brand_color) VALUES ($1,$2,'Manual Reply Cancel Test','#7c3aed')`,
		studioID, "manual-reply-cancel-test-"+studioID.String()[:8]); err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM studios WHERE id = $1`, studioID) })

	userID = uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, studio_id, email, password_hash, role) VALUES ($1,$2,$3,'x','studio_admin')`,
		userID, studioID, "manual-reply-cancel-test-"+userID.String()[:8]+"@example.com"); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	channelID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO channel_accounts (id, studio_id, kind, external_id, display_handle, access_token_enc) VALUES ($1,$2,'whatsapp_web','test-channel','Test','enc')`,
		channelID, studioID); err != nil {
		t.Fatalf("create test channel: %v", err)
	}
	contactID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO contact_identities (id, studio_id, kind, value, display_name) VALUES ($1,$2,'phone','6591234567','Test Contact')`,
		contactID, studioID); err != nil {
		t.Fatalf("create test contact: %v", err)
	}
	conv := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO conversations (id, studio_id, channel_account_id, contact_identity_id, external_thread_id, ai_enabled) VALUES ($1,$2,$3,$4,'test-thread',true)`,
		conv, studioID, channelID, contactID); err != nil {
		t.Fatalf("create test conversation: %v", err)
	}
	convID = conv

	msgRepo := NewRepo(pool, nil)
	svc = NewService(msgRepo, NewInProcBus(), "", "", nil, nil, "")
	return svc, pool, studioID, convID, userID
}

// Real scenario reported: a staff member replies manually to a lead in the
// Inbox, but a "Manual Actions" job (or a queued AI follow-up) scheduled
// earlier for that same lead was completely unaffected — it would still
// fire later, landing on top of the reply the staff member just sent. A
// manual reply must now cancel any other still-pending automated/AI job for
// that conversation (DELETEs the row, so existence — not status — is what's
// checked here), but never touch a studio_user-sourced one, including the
// reply that was just sent.
func TestEnqueueReply_CancelsPendingAutomatedJobsButNotStudioUserOnes(t *testing.T) {
	svc, pool, studioID, convID, userID := setupManualReplyCancelTest(t)
	ctx := context.Background()

	insertJob := func(sourceKind, sourceRef string) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `
			INSERT INTO outbound_jobs (studio_id, conversation_id, body, source_kind, source_ref, status)
			VALUES ($1,$2,'queued message',$3,$4,'pending')
			RETURNING id
		`, studioID, convID, sourceKind, sourceRef).Scan(&id); err != nil {
			t.Fatalf("insert %s job: %v", sourceKind, err)
		}
		return id
	}

	manualActionID := insertJob("automation", "manual_action:test")
	aiFollowupID := insertJob("ai", "lead:test:followup:1")
	otherStaffReplyID := insertJob("studio_user", "")

	replyJobID, err := svc.EnqueueReply(ctx, SendInput{StudioID: studioID, ConversationID: convID, UserID: userID, Body: "Hi, following up personally!"})
	if err != nil {
		t.Fatalf("EnqueueReply: %v", err)
	}

	exists := func(id int64) bool {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbound_jobs WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatalf("check job %d existence: %v", id, err)
		}
		return n == 1
	}

	if exists(manualActionID) {
		t.Error("pending Manual Actions job still exists after a manual reply — it should have been cancelled")
	}
	if exists(aiFollowupID) {
		t.Error("pending AI follow-up job still exists after a manual reply — it should have been cancelled")
	}
	if !exists(otherStaffReplyID) {
		t.Error("a different pending studio_user job was deleted — manual reply must only cancel automated/AI jobs")
	}
	if !exists(replyJobID) {
		t.Error("the manual reply's own just-enqueued job was deleted — it must never cancel itself")
	}
}
