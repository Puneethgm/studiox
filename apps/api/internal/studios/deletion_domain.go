package studios

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// DeletionRequest mirrors studio_deletion_requests (migration
// 20261010010001_studio_deletion_requests.sql) — the confirmation-gate
// step of the account-deletion flow (init.md §4.5).
type DeletionRequest struct {
	ID                uuid.UUID
	StudioID          uuid.UUID
	RequestedByUserID uuid.UUID
	ConfirmTokenHash  string
	Status            string
	RequestedAt       time.Time
	ConfirmedAt       *time.Time
	CompletedAt       *time.Time
	ExpiresAt         time.Time
}

// AccountArchive mirrors account_archives (migration
// 20261010020001_account_archives.sql) — the encrypted, recoverable export
// created at deletion time (init.md §4.5's "recovery link").
type AccountArchive struct {
	ID                uuid.UUID
	StudioID          uuid.UUID
	S3Key             string
	WrappedDataKey    string
	SizeBytes         int64
	RecoveryTokenHash string
	RecoveryEmail     string
	RecoveryExpiresAt time.Time
	DownloadedAt      *time.Time
	CreatedAt         time.Time
}

var (
	ErrEmailMismatch               = errors.New("email does not match studio contact email")
	ErrDeletionRequestNotFound     = errors.New("deletion request not found")
	ErrDeletionRequestInvalidState = errors.New("deletion request is not in a valid state for this action")
	ErrDeletionRequestExpired      = errors.New("deletion confirmation link has expired")
	ErrArchiveNotFound             = errors.New("account archive not found")
	ErrArchiveExpired              = errors.New("account archive recovery link has expired")
	ErrArchiveEmailMismatch        = errors.New("email does not match the archive's recovery email")
)
