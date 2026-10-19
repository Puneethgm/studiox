package studios

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/projectx/api/internal/platform/audit"
	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/mail"
	"github.com/projectx/api/internal/platform/secrets"
)

// TestAccountDeletionRequestHappyPath exercises the request-deletion step
// (Service.RequestAccountDeletion) against a real Postgres database,
// following the same .env-driven connect-or-skip pattern as this package's
// setupEscalationTestConversation and internal/leads/dnd_test.go's testDB:
// create a throwaway studio, request its deletion, and verify both the
// studio_deletion_requests row and the audit_logs row it's supposed to
// produce. It also exercises the email-mismatch rejection path.
//
// This test deliberately stops at the request step and does not call
// ConfirmAccountDeletion/RecoverArchive end to end: those two call real S3
// (UploadPrivate/Download) through *s3.Uploader, which has no interface/
// mock seam in this codebase today, and the only S3 bucket configured in
// this environment's .env is named "...-prod" — this test will not write
// archive objects into a production bucket. The confirm/recover request
// validation logic (token hash lookup, status/expiry checks, email
// matching) is covered without S3 by TestConfirmAndRecoverValidationPaths
// below; the S3-dependent happy path was verified by code review plus
// `go build`/`go vet`, not by an automated run — see the task summary.
func TestAccountDeletionRequestHappyPath(t *testing.T) {
	pool, cleanup := connectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	studioID, contactEmail := createTestStudioForDeletion(t, ctx, pool)
	actorID := uuid.New()

	svc := newDeletionTestService(t, pool)

	// Wrong email must be rejected before anything is written.
	err := svc.RequestAccountDeletion(ctx, studioID, actorID, "someone-else@example.com")
	if !errors.Is(err, ErrEmailMismatch) {
		t.Fatalf("expected ErrEmailMismatch, got %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM studio_deletion_requests WHERE studio_id = $1`, studioID).Scan(&count); err != nil {
		t.Fatalf("count deletion requests: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no deletion request row after a rejected email mismatch, got %d", count)
	}

	// Correct email creates a pending request + audit row.
	if err := svc.RequestAccountDeletion(ctx, studioID, actorID, contactEmail); err != nil {
		t.Fatalf("RequestAccountDeletion: %v", err)
	}

	var status, tokenHash string
	var expiresAt time.Time
	err = pool.QueryRow(ctx, `
		SELECT status, confirm_token_hash, expires_at FROM studio_deletion_requests WHERE studio_id = $1
	`, studioID).Scan(&status, &tokenHash, &expiresAt)
	if err != nil {
		t.Fatalf("query deletion request: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected status=pending, got %q", status)
	}
	if tokenHash == "" {
		t.Fatal("expected a non-empty confirm_token_hash (the raw token itself must never be stored)")
	}
	if time.Until(expiresAt) > 25*time.Hour || time.Until(expiresAt) < 23*time.Hour {
		t.Fatalf("expected expires_at ~24h from now, got %v from now", time.Until(expiresAt))
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs WHERE studio_id = $1 AND action = 'deletion_requested'
	`, studioID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected exactly one deletion_requested audit row, got %d", auditCount)
	}
}

// TestConfirmAndRecoverValidationPaths exercises ConfirmAccountDeletion's
// and RecoverArchive's token/status/expiry/email validation against a real
// Postgres database, without going anywhere near S3: every case here is
// rejected before the code would reach the S3 upload/download calls, so no
// *s3.Uploader is even configured on the service (it's nil), proving these
// guard clauses run — and fail safe — independently of S3 availability.
func TestConfirmAndRecoverValidationPaths(t *testing.T) {
	pool, cleanup := connectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	svc := newDeletionTestService(t, pool)

	t.Run("confirm: unknown token", func(t *testing.T) {
		err := svc.ConfirmAccountDeletion(ctx, "not-a-real-token")
		if !errors.Is(err, ErrDeletionRequestNotFound) {
			t.Fatalf("expected ErrDeletionRequestNotFound, got %v", err)
		}
	})

	t.Run("confirm: expired token", func(t *testing.T) {
		studioID, _ := createTestStudioForDeletion(t, ctx, pool)
		rawToken, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO studio_deletion_requests (studio_id, requested_by_user_id, confirm_token_hash, status, expires_at)
			VALUES ($1, $2, $3, 'pending', now() - interval '1 hour')
		`, studioID, uuid.New(), hashToken(rawToken))
		if err != nil {
			t.Fatalf("insert expired deletion request: %v", err)
		}

		err = svc.ConfirmAccountDeletion(ctx, rawToken)
		if !errors.Is(err, ErrDeletionRequestExpired) {
			t.Fatalf("expected ErrDeletionRequestExpired, got %v", err)
		}
	})

	t.Run("confirm: already-completed token", func(t *testing.T) {
		studioID, _ := createTestStudioForDeletion(t, ctx, pool)
		rawToken, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO studio_deletion_requests (studio_id, requested_by_user_id, confirm_token_hash, status, expires_at)
			VALUES ($1, $2, $3, 'completed', now() + interval '1 hour')
		`, studioID, uuid.New(), hashToken(rawToken))
		if err != nil {
			t.Fatalf("insert completed deletion request: %v", err)
		}

		err = svc.ConfirmAccountDeletion(ctx, rawToken)
		if !errors.Is(err, ErrDeletionRequestInvalidState) {
			t.Fatalf("expected ErrDeletionRequestInvalidState, got %v", err)
		}
	})

	t.Run("recover: unknown token", func(t *testing.T) {
		_, err := svc.RecoverArchive(ctx, "not-a-real-token", "someone@example.com")
		if !errors.Is(err, ErrArchiveNotFound) {
			t.Fatalf("expected ErrArchiveNotFound, got %v", err)
		}
	})

	t.Run("recover: expired archive", func(t *testing.T) {
		studioID, email := createTestStudioForDeletion(t, ctx, pool)
		rawToken, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO account_archives (studio_id, s3_key, wrapped_data_key, size_bytes, recovery_token_hash, recovery_email, recovery_expires_at)
			VALUES ($1, 'archives/test/fake.enc', 'fake-wrapped-key', 10, $2, $3, now() - interval '1 hour')
		`, studioID, hashToken(rawToken), email)
		if err != nil {
			t.Fatalf("insert expired archive: %v", err)
		}

		_, err = svc.RecoverArchive(ctx, rawToken, email)
		if !errors.Is(err, ErrArchiveExpired) {
			t.Fatalf("expected ErrArchiveExpired, got %v", err)
		}
		assertAuditRowExists(t, ctx, pool, studioID, "recovery_download_attempt")
	})

	t.Run("recover: email mismatch", func(t *testing.T) {
		studioID, email := createTestStudioForDeletion(t, ctx, pool)
		rawToken, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO account_archives (studio_id, s3_key, wrapped_data_key, size_bytes, recovery_token_hash, recovery_email, recovery_expires_at)
			VALUES ($1, 'archives/test/fake.enc', 'fake-wrapped-key', 10, $2, $3, now() + interval '1 hour')
		`, studioID, hashToken(rawToken), email)
		if err != nil {
			t.Fatalf("insert archive: %v", err)
		}

		_, err = svc.RecoverArchive(ctx, rawToken, "wrong-email@example.com")
		if !errors.Is(err, ErrArchiveEmailMismatch) {
			t.Fatalf("expected ErrArchiveEmailMismatch, got %v", err)
		}
		assertAuditRowExists(t, ctx, pool, studioID, "recovery_download_attempt")
	})
}

// assertAuditRowExists checks for the audit row *before* the calling
// subtest returns — createTestStudioForDeletion registers a t.Cleanup that
// deletes that studio's audit_logs rows, and subtest cleanups run as soon
// as the subtest function returns, not at the end of the parent test, so
// this must be called from inside the subtest body.
func assertAuditRowExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, studioID uuid.UUID, action string) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs WHERE studio_id = $1 AND action = $2
	`, studioID, action).Scan(&count); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least one %q audit row for studio %s, got %d", action, studioID, count)
	}
}

// TestBuildArchiveAgainstRealSchema runs BuildArchive against the real
// Postgres schema (not just in-memory encryption, which
// TestArchiveEncryptDecryptRoundTrip already covers) to prove the
// row_to_json-based export SQL actually matches the live studios/users/
// leads/campaigns/conversations/messages columns, rather than just
// type-checking against a guessed column list. Creates a throwaway studio
// with one user and one lead, builds the archive, and verifies it unzips
// to the expected manifest with the right row counts.
func TestBuildArchiveAgainstRealSchema(t *testing.T) {
	pool, cleanup := connectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	studioID, _ := createTestStudioForDeletion(t, ctx, pool)

	userID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, studio_id, email, password_hash, role)
		VALUES ($1, $2, $3, 'test-hash', 'studio_admin')
	`, userID, studioID, fmt.Sprintf("user-%s@example.com", userID.String()[:8])); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	campaignID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO campaigns (id, studio_id, slug, name, created_by)
		VALUES ($1, $2, 'test-campaign', 'Test Campaign', $3)
	`, campaignID, studioID, userID); err != nil {
		t.Fatalf("create test campaign: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM campaigns WHERE id = $1`, campaignID) })

	leadID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO leads (id, studio_id, campaign_id, name, email, phone, fitness_plan)
		VALUES ($1, $2, $3, 'Test Lead', 'lead@example.com', '+10000000000', 'weight-loss')
	`, leadID, studioID, campaignID); err != nil {
		t.Fatalf("create test lead: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM leads WHERE id = $1`, leadID) })

	raw, err := BuildArchive(ctx, pool, studioID)
	if err != nil {
		t.Fatalf("BuildArchive: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty archive bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open archive as zip: %v", err)
	}
	names := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		names[f.Name] = f
	}
	for _, want := range []string{"studios.json", "users.json", "leads.json", "campaigns.json", "conversations.json", "messages.json", "manifest.json", "files.json"} {
		if _, ok := names[want]; !ok {
			t.Errorf("expected %s to be present in the archive", want)
		}
	}

	manifestFile, ok := names["manifest.json"]
	if !ok {
		t.Fatal("manifest.json missing")
	}
	mf, err := manifestFile.Open()
	if err != nil {
		t.Fatalf("open manifest.json: %v", err)
	}
	defer mf.Close()
	var manifest []archiveManifestEntry
	if err := json.NewDecoder(mf).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest.json: %v", err)
	}
	rowsByTable := make(map[string]int, len(manifest))
	for _, e := range manifest {
		rowsByTable[e.Table] = e.Rows
	}
	if rowsByTable["studios"] != 1 {
		t.Errorf("expected 1 studios row, got %d", rowsByTable["studios"])
	}
	if rowsByTable["users"] != 1 {
		t.Errorf("expected 1 users row, got %d", rowsByTable["users"])
	}
	if rowsByTable["leads"] != 1 {
		t.Errorf("expected 1 leads row, got %d", rowsByTable["leads"])
	}
	if rowsByTable["campaigns"] != 1 {
		t.Errorf("expected 1 campaigns row, got %d", rowsByTable["campaigns"])
	}
}

// ----- shared test helpers -----

func connectTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	// godotenv.Load aborts on the first path that doesn't exist when given
	// several filenames in one call, so each candidate is loaded in its own
	// call (same pattern as config.Load) rather than as one variadic list.
	for _, path := range []string{"../../.env", "../../../.env", "../../../../.env"} {
		_ = godotenv.Load(path)
	}
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
	return pool, pool.Close
}

func createTestStudioForDeletion(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (studioID uuid.UUID, contactEmail string) {
	t.Helper()
	studioID = uuid.New()
	contactEmail = fmt.Sprintf("owner-%s@example.com", studioID.String()[:8])
	_, err := pool.Exec(ctx, `
		INSERT INTO studios (id, slug, name, brand_color, contact_email)
		VALUES ($1, $2, 'Deletion Flow Test', '#7c3aed', $3)
	`, studioID, "deletion-test-"+studioID.String()[:8], contactEmail)
	if err != nil {
		t.Fatalf("create test studio: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		// studio_deletion_requests/account_archives/audit_logs have no FK
		// to studios (by design — see their migrations), so they need an
		// explicit cleanup instead of relying on cascade.
		_, _ = pool.Exec(bg, `DELETE FROM audit_logs WHERE studio_id = $1`, studioID)
		_, _ = pool.Exec(bg, `DELETE FROM account_archives WHERE studio_id = $1`, studioID)
		_, _ = pool.Exec(bg, `DELETE FROM studio_deletion_requests WHERE studio_id = $1`, studioID)
		_, _ = pool.Exec(bg, `DELETE FROM studios WHERE id = $1`, studioID)
	})
	return studioID, contactEmail
}

func newDeletionTestService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	keyBytes := make([]byte, secrets.RequiredKeyBytes)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("generate test cipher key: %v", err)
	}
	cipher, err := secrets.New(base64.StdEncoding.EncodeToString(keyBytes))
	if err != nil {
		t.Fatalf("new test cipher: %v", err)
	}
	repo := NewRepo(pool, cipher)
	auditLogger := audit.New(pool)
	// SMTP left unconfigured on purpose — mailer.Enabled() is false, so
	// RequestAccountDeletion/ConfirmAccountDeletion skip the actual send
	// and this test never depends on a real mail server.
	mailer := mail.NewSender(config.SMTPConfig{})
	return NewService(repo, nil, nil, nil, nil, mailer, "http://localhost:3000", nil, auditLogger)
}
