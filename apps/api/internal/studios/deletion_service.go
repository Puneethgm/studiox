package studios

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	// deletionConfirmTokenTTL is how long the emailed confirm link stays
	// valid — init.md §4.5 doesn't pin an exact number for this step, 24h
	// mirrors the other short-lived single-use links in this codebase
	// (password reset, teammate welcome) being measured in hours, not days.
	deletionConfirmTokenTTL = 24 * time.Hour
	// archiveRecoveryTTL is the 7-day recovery-link validity window
	// required by init.md §4.5.
	archiveRecoveryTTL = 7 * 24 * time.Hour
	// paymentRetentionPeriod is the 7-year financial-record retention
	// window required by init.md §4.5.
	paymentRetentionPeriod = 7 * 365 * 24 * time.Hour
)

// newRawToken generates a fresh, high-entropy, hex-encoded single-use
// token (32 random bytes -> 64 hex chars) for both the deletion-confirm
// link and the archive-recovery link. Only the sha256 hash of this value
// is ever persisted — see hashToken.
func newRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// logAudit is the fire-and-forget-but-logged pattern every step of the
// deletion/recovery flow uses: write the audit row, and if that write
// itself fails, log it and move on — an audit-write failure must never
// abort the primary operation (see internal/platform/audit's doc comment).
func (s *Service) logAudit(ctx context.Context, studioID, actorUserID *uuid.UUID, action string, metadata map[string]any) {
	if s.auditLogger == nil {
		return
	}
	if err := s.auditLogger.Log(ctx, studioID, actorUserID, action, metadata); err != nil {
		slog.Error("audit log write failed", "action", action, "err", err)
	}
}

// RequestAccountDeletion is step 1 of init.md §4.5's compliant deletion
// flow: validates the caller-supplied email against the studio's contact
// email, creates a pending studio_deletion_requests row with a hashed,
// single-use, 24-hour confirm token, and emails the raw token (never
// stored) to the studio's contact address as a confirmation link.
// actorUserID is recorded on the request and in the audit log for
// traceability. The "I am sure" checkbox itself is enforced by the HTTP
// handler before this is ever called (http.go's requestAccountDeletion).
func (s *Service) RequestAccountDeletion(ctx context.Context, studioID, actorUserID uuid.UUID, email string) error {
	studio, err := s.repo.GetByID(ctx, studioID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(email), studio.ContactEmail) {
		return ErrEmailMismatch
	}

	rawToken, err := newRawToken()
	if err != nil {
		return fmt.Errorf("generate confirm token: %w", err)
	}
	tokenHash := hashToken(rawToken)
	expiresAt := time.Now().Add(deletionConfirmTokenTTL)

	dr, err := s.repo.CreateDeletionRequest(ctx, studioID, actorUserID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create deletion request: %w", err)
	}

	s.logAudit(ctx, &studioID, &actorUserID, "deletion_requested", map[string]any{
		"deletion_request_id": dr.ID,
		"expires_at":          expiresAt,
	})

	if s.mailer != nil && s.mailer.Enabled() {
		confirmLink := fmt.Sprintf("%s/account/delete/confirm?token=%s", s.frontendURL, rawToken)
		studioName := studio.Name
		contactEmail := studio.ContactEmail
		deletionRequestID := dr.ID
		go func() {
			bgCtx := context.Background()
			if err := s.mailer.SendAccountDeletionPending(contactEmail, studioName, confirmLink); err != nil {
				slog.Error("account deletion pending email failed", "err", err, "studio_id", studioID)
				return
			}
			s.logAudit(bgCtx, &studioID, &actorUserID, "pre_deletion_email_sent", map[string]any{
				"deletion_request_id": deletionRequestID,
			})
		}()
	} else {
		slog.Warn("account deletion pending email skipped — SMTP not configured", "studio_id", studioID)
	}

	return nil
}

// ConfirmAccountDeletion is step 2 — the "final verification" of init.md
// §4.5. Given the raw token from the emailed confirm link, it validates it
// against the stored hash, then synchronously runs the whole
// backup-then-delete sequence.
//
// Ordering is deliberate: the encrypted archive must be durably stored in
// S3 BEFORE the studio row is deleted, so a failure partway through never
// deletes data with no backup in place. If the archive build/encrypt/
// upload step fails, this returns an error and nothing is deleted. Once
// the archive is safely in S3, the remaining steps (payment-record copy,
// archive-row insert, studio delete, request completion) run inside a
// single DB transaction — if any of those fail, the transaction rolls
// back and the studio is NOT deleted (an orphaned archive upload in S3 is
// the acceptable cost of that safety margin, not data loss).
func (s *Service) ConfirmAccountDeletion(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	dr, err := s.repo.GetDeletionRequestByTokenHash(ctx, tokenHash)
	if err != nil {
		return err
	}
	if dr.Status != "pending" {
		return ErrDeletionRequestInvalidState
	}
	if time.Now().After(dr.ExpiresAt) {
		return ErrDeletionRequestExpired
	}

	if err := s.repo.MarkDeletionRequestConfirmed(ctx, dr.ID); err != nil {
		return err
	}
	s.logAudit(ctx, &dr.StudioID, &dr.RequestedByUserID, "deletion_confirmed", map[string]any{
		"deletion_request_id": dr.ID,
	})

	studio, err := s.repo.GetByID(ctx, dr.StudioID)
	if err != nil {
		return fmt.Errorf("load studio before deletion: %w", err)
	}
	studioName := studio.Name
	contactEmail := studio.ContactEmail

	// --- backup: build + encrypt + upload BEFORE anything is deleted ---
	raw, err := BuildArchive(ctx, s.repo.Pool(), dr.StudioID)
	if err != nil {
		return fmt.Errorf("build archive: %w", err)
	}
	encrypted, wrappedKey, err := EncryptArchive(raw, s.repo.Cipher())
	if err != nil {
		return fmt.Errorf("encrypt archive: %w", err)
	}
	if s.s3Uploader == nil {
		return fmt.Errorf("account deletion aborted: archive storage (S3) is not configured")
	}
	s3Key := fmt.Sprintf("archives/%s/%d.enc", dr.StudioID, time.Now().Unix())
	if err := s.s3Uploader.UploadPrivate(ctx, s3Key, encrypted, "application/octet-stream"); err != nil {
		return fmt.Errorf("upload archive: %w", err)
	}
	s.logAudit(ctx, &dr.StudioID, &dr.RequestedByUserID, "archive_created", map[string]any{
		"s3_key": s3Key, "size_bytes": len(encrypted),
	})

	rawRecoveryToken, err := newRawToken()
	if err != nil {
		return fmt.Errorf("generate recovery token: %w", err)
	}
	recoveryTokenHash := hashToken(rawRecoveryToken)
	recoveryExpiresAt := time.Now().Add(archiveRecoveryTTL)

	// --- DB-only from here: payment retention copy, archive row, studio
	// delete, request completion — all in one transaction, now that the
	// backup is durably stored outside it.
	pool := s.repo.Pool()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin deletion tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.CopyPaymentRecordsForRetention(ctx, tx, dr.StudioID, time.Now().Add(paymentRetentionPeriod)); err != nil {
		return fmt.Errorf("copy payment records: %w", err)
	}

	var archiveID uuid.UUID
	row := tx.QueryRow(ctx, `
		INSERT INTO account_archives (studio_id, s3_key, wrapped_data_key, size_bytes, recovery_token_hash, recovery_email, recovery_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, dr.StudioID, s3Key, wrappedKey, int64(len(encrypted)), recoveryTokenHash, contactEmail, recoveryExpiresAt)
	if err := row.Scan(&archiveID); err != nil {
		return fmt.Errorf("insert account archive: %w", err)
	}

	if err := s.repo.HardDeleteStudio(ctx, tx, dr.StudioID); err != nil {
		return fmt.Errorf("delete studio: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE studio_deletion_requests SET status = 'completed', completed_at = now() WHERE id = $1
	`, dr.ID); err != nil {
		return fmt.Errorf("mark deletion request completed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit deletion tx: %w", err)
	}

	// Audit logging and the post-deletion email happen after commit, using
	// a background context — the studio row is already gone and the HTTP
	// response shouldn't wait on these.
	s.logAudit(context.Background(), &dr.StudioID, &dr.RequestedByUserID, "account_deleted", map[string]any{
		"deletion_request_id": dr.ID,
	})
	s.logAudit(context.Background(), &dr.StudioID, &dr.RequestedByUserID, "recovery_link_generated", map[string]any{
		"archive_id": archiveID, "expires_at": recoveryExpiresAt,
	})

	if s.mailer != nil && s.mailer.Enabled() {
		recoveryLink := fmt.Sprintf("%s/recover?token=%s", s.frontendURL, rawRecoveryToken)
		if err := s.mailer.SendAccountDeletedConfirmation(contactEmail, studioName, recoveryLink); err != nil {
			slog.Error("post-deletion confirmation email failed", "err", err, "studio_id", dr.StudioID)
		} else {
			s.logAudit(context.Background(), &dr.StudioID, &dr.RequestedByUserID, "post_deletion_email_sent", map[string]any{
				"archive_id": archiveID,
			})
		}
	} else {
		slog.Warn("post-deletion confirmation email skipped — SMTP not configured", "studio_id", dr.StudioID)
	}

	return nil
}

// RecoverArchive is the public recovery/download step of init.md §4.5:
// looks up the archive by the recovery token's hash, verifies it hasn't
// expired and that the supplied email matches the contact-email snapshot
// taken at deletion time (this is the "proper authentication/verification"
// the spec asks for, without requiring a password for an account that no
// longer exists), then downloads, decrypts, and returns the archive bytes.
//
// Every attempt — success or failure — is audit-logged. Failures return a
// generic sentinel error to the caller (http.go maps every case to the
// same 400 response) so a wrong guess never reveals which specific check
// failed, matching this codebase's existing "don't reveal account
// existence" posture (see platform/mail's package doc comment re:
// password reset).
func (s *Service) RecoverArchive(ctx context.Context, rawToken, email string) ([]byte, error) {
	tokenHash := hashToken(rawToken)
	archive, err := s.repo.GetArchiveByTokenHash(ctx, tokenHash)
	if err != nil {
		s.logAudit(ctx, nil, nil, "recovery_download_attempt", map[string]any{"reason": "token_not_found"})
		return nil, ErrArchiveNotFound
	}
	if time.Now().After(archive.RecoveryExpiresAt) {
		s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_attempt", map[string]any{
			"reason": "expired", "archive_id": archive.ID,
		})
		return nil, ErrArchiveExpired
	}
	if !strings.EqualFold(strings.TrimSpace(email), archive.RecoveryEmail) {
		s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_attempt", map[string]any{
			"reason": "email_mismatch", "archive_id": archive.ID,
		})
		return nil, ErrArchiveEmailMismatch
	}

	if s.s3Uploader == nil {
		s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_attempt", map[string]any{
			"reason": "s3_unavailable", "archive_id": archive.ID,
		})
		return nil, errors.New("archive storage is not available")
	}
	encrypted, err := s.s3Uploader.Download(ctx, archive.S3Key)
	if err != nil {
		s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_attempt", map[string]any{
			"reason": "s3_download_failed", "archive_id": archive.ID,
		})
		return nil, fmt.Errorf("download archive: %w", err)
	}
	raw, err := DecryptArchive(encrypted, archive.WrappedDataKey, s.repo.Cipher())
	if err != nil {
		s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_attempt", map[string]any{
			"reason": "decrypt_failed", "archive_id": archive.ID,
		})
		return nil, fmt.Errorf("decrypt archive: %w", err)
	}

	if err := s.repo.MarkArchiveDownloaded(ctx, archive.ID); err != nil {
		slog.Warn("mark archive downloaded failed", "err", err, "archive_id", archive.ID)
	}
	s.logAudit(ctx, &archive.StudioID, nil, "recovery_download_success", map[string]any{"archive_id": archive.ID})

	return raw, nil
}
