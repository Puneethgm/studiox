package studios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/platform/secrets"
)

// Cipher exposes the repo's existing master cipher so the account-deletion
// archive flow (archive.go's EncryptArchive/DecryptArchive) can reuse the
// exact same instance this repo already uses for Stripe secrets, instead
// of constructing a second one.
func (r *Repo) Cipher() *secrets.Cipher { return r.cipher }

// CreateDeletionRequest inserts a new pending studio_deletion_requests row
// — step 1 of the account-deletion flow (init.md §4.5).
func (r *Repo) CreateDeletionRequest(ctx context.Context, studioID, requestedByUserID uuid.UUID, confirmTokenHash string, expiresAt time.Time) (*DeletionRequest, error) {
	dr := &DeletionRequest{
		StudioID:          studioID,
		RequestedByUserID: requestedByUserID,
		ConfirmTokenHash:  confirmTokenHash,
		Status:            "pending",
		ExpiresAt:         expiresAt,
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO studio_deletion_requests (studio_id, requested_by_user_id, confirm_token_hash, status, expires_at)
		VALUES ($1, $2, $3, 'pending', $4)
		RETURNING id, requested_at
	`, studioID, requestedByUserID, confirmTokenHash, expiresAt)
	if err := row.Scan(&dr.ID, &dr.RequestedAt); err != nil {
		return nil, fmt.Errorf("insert deletion request: %w", err)
	}
	return dr, nil
}

// GetDeletionRequestByTokenHash looks up a deletion request by the sha256
// hash of the raw confirm token from the emailed link — the raw token is
// never stored, only its hash.
func (r *Repo) GetDeletionRequestByTokenHash(ctx context.Context, tokenHash string) (*DeletionRequest, error) {
	var dr DeletionRequest
	err := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, requested_by_user_id, confirm_token_hash, status, requested_at, confirmed_at, completed_at, expires_at
		FROM studio_deletion_requests WHERE confirm_token_hash = $1
	`, tokenHash).Scan(&dr.ID, &dr.StudioID, &dr.RequestedByUserID, &dr.ConfirmTokenHash, &dr.Status,
		&dr.RequestedAt, &dr.ConfirmedAt, &dr.CompletedAt, &dr.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeletionRequestNotFound
		}
		return nil, fmt.Errorf("get deletion request: %w", err)
	}
	return &dr, nil
}

// MarkDeletionRequestConfirmed moves a request from pending to confirmed —
// atomically guarded by the WHERE clause so a token can't be "confirmed"
// twice (double-submit of the same emailed link).
func (r *Repo) MarkDeletionRequestConfirmed(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE studio_deletion_requests SET status = 'confirmed', confirmed_at = now()
		WHERE id = $1 AND status = 'pending'
	`, id)
	if err != nil {
		return fmt.Errorf("mark deletion request confirmed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeletionRequestInvalidState
	}
	return nil
}

// CopyPaymentRecordsForRetention copies the minimal fields required for
// the 7-year financial-record retention window (init.md §4.5) out of
// studio_subscriptions and user_subscriptions into
// retained_payment_records, run as part of the same transaction that
// deletes the studio so the copy and the deletion either both happen or
// neither does. Each source row's full contents are also kept as
// raw_snapshot, beyond the fields promoted into their own columns.
//
// processed_stripe_events is deliberately NOT copied here: it has no
// studio_id column (it's keyed only by Stripe event ID), so there is no
// studio-scoped way to select "this studio's rows" out of it.
func (r *Repo) CopyPaymentRecordsForRetention(ctx context.Context, tx pgx.Tx, studioID uuid.UUID, retainUntil time.Time) error {
	type studioSub struct {
		ID               uuid.UUID `json:"id"`
		StripeSubID      string    `json:"stripeSubId"`
		StripeCustomerID string    `json:"stripeCustomerId"`
		PlanTier         string    `json:"planTier"`
		Status           string    `json:"status"`
		PriceUSD         int       `json:"priceUsd"`
		CurrentPeriodEnd time.Time `json:"currentPeriodEnd"`
		CreatedAt        time.Time `json:"createdAt"`
		UpdatedAt        time.Time `json:"updatedAt"`
	}
	subRows, err := tx.Query(ctx, `
		SELECT id, stripe_sub_id, stripe_customer_id, plan_tier, status, price_usd, current_period_end, created_at, updated_at
		FROM studio_subscriptions WHERE studio_id = $1
	`, studioID)
	if err != nil {
		return fmt.Errorf("select studio_subscriptions: %w", err)
	}
	var subs []studioSub
	for subRows.Next() {
		var s studioSub
		if err := subRows.Scan(&s.ID, &s.StripeSubID, &s.StripeCustomerID, &s.PlanTier, &s.Status,
			&s.PriceUSD, &s.CurrentPeriodEnd, &s.CreatedAt, &s.UpdatedAt); err != nil {
			subRows.Close()
			return fmt.Errorf("scan studio_subscriptions: %w", err)
		}
		subs = append(subs, s)
	}
	subRows.Close()
	if err := subRows.Err(); err != nil {
		return fmt.Errorf("iterate studio_subscriptions: %w", err)
	}
	for _, s := range subs {
		snapshot, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("marshal studio_subscriptions snapshot: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO retained_payment_records
				(studio_id, stripe_customer_id, stripe_subscription_id, plan_tier, price_usd, period_start, period_end, raw_snapshot, retain_until)
			VALUES ($1, $2, $3, $4, $5, NULL, $6, $7, $8)
		`, studioID, s.StripeCustomerID, s.StripeSubID, s.PlanTier, s.PriceUSD, s.CurrentPeriodEnd, snapshot, retainUntil)
		if err != nil {
			return fmt.Errorf("insert retained payment record (studio_subscriptions): %w", err)
		}
	}

	type userSub struct {
		ID                 uuid.UUID  `json:"id"`
		LeadID             uuid.UUID  `json:"leadId"`
		PlanID             uuid.UUID  `json:"planId"`
		PlanName           string     `json:"planName"`
		AmountPaid         int        `json:"amountPaid"`
		Currency           string     `json:"currency"`
		PaymentID          string     `json:"paymentId"`
		PaymentStatus      string     `json:"paymentStatus"`
		SubscriptionStatus string     `json:"subscriptionStatus"`
		StartDate          time.Time  `json:"startDate"`
		ExpiryDate         *time.Time `json:"expiryDate,omitempty"`
		CreatedAt          time.Time  `json:"createdAt"`
		UpdatedAt          time.Time  `json:"updatedAt"`
	}
	userSubRows, err := tx.Query(ctx, `
		SELECT id, lead_id, plan_id, plan_name, amount_paid, currency, payment_id, payment_status, subscription_status, start_date, expiry_date, created_at, updated_at
		FROM user_subscriptions WHERE studio_id = $1
	`, studioID)
	if err != nil {
		return fmt.Errorf("select user_subscriptions: %w", err)
	}
	var userSubs []userSub
	for userSubRows.Next() {
		var s userSub
		if err := userSubRows.Scan(&s.ID, &s.LeadID, &s.PlanID, &s.PlanName, &s.AmountPaid, &s.Currency,
			&s.PaymentID, &s.PaymentStatus, &s.SubscriptionStatus, &s.StartDate, &s.ExpiryDate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			userSubRows.Close()
			return fmt.Errorf("scan user_subscriptions: %w", err)
		}
		userSubs = append(userSubs, s)
	}
	userSubRows.Close()
	if err := userSubRows.Err(); err != nil {
		return fmt.Errorf("iterate user_subscriptions: %w", err)
	}
	for _, s := range userSubs {
		snapshot, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("marshal user_subscriptions snapshot: %w", err)
		}
		// amount_paid/currency on user_subscriptions is the studio's own
		// membership-plan price (SGD today, see leads/plans), not
		// necessarily USD — price_usd here is just the shared numeric
		// retention column name from the spec; the real currency is kept
		// in raw_snapshot.
		priceUSD := float64(s.AmountPaid) / 100.0
		_, err = tx.Exec(ctx, `
			INSERT INTO retained_payment_records
				(studio_id, stripe_customer_id, stripe_subscription_id, plan_tier, price_usd, period_start, period_end, raw_snapshot, retain_until)
			VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8)
		`, studioID, s.PaymentID, s.PlanName, priceUSD, s.StartDate, s.ExpiryDate, snapshot, retainUntil)
		if err != nil {
			return fmt.Errorf("insert retained payment record (user_subscriptions): %w", err)
		}
	}

	return nil
}

// GetArchiveByTokenHash looks up an account_archives row by the sha256 hash
// of the raw recovery token from the emailed link.
func (r *Repo) GetArchiveByTokenHash(ctx context.Context, tokenHash string) (*AccountArchive, error) {
	var a AccountArchive
	err := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, s3_key, wrapped_data_key, size_bytes, recovery_token_hash, recovery_email, recovery_expires_at, downloaded_at, created_at
		FROM account_archives WHERE recovery_token_hash = $1
	`, tokenHash).Scan(&a.ID, &a.StudioID, &a.S3Key, &a.WrappedDataKey, &a.SizeBytes, &a.RecoveryTokenHash,
		&a.RecoveryEmail, &a.RecoveryExpiresAt, &a.DownloadedAt, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrArchiveNotFound
		}
		return nil, fmt.Errorf("get account archive: %w", err)
	}
	return &a, nil
}

// MarkArchiveDownloaded records the first successful recovery download.
// Repeat downloads within the validity window are deliberately NOT
// blocked (per spec) — this just timestamps when it first happened, hence
// the `AND downloaded_at IS NULL` guard so later calls are harmless no-ops.
func (r *Repo) MarkArchiveDownloaded(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE account_archives SET downloaded_at = now() WHERE id = $1 AND downloaded_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("mark archive downloaded: %w", err)
	}
	return nil
}

// HardDeleteStudio permanently deletes the studio row (cascading via the
// same FK constraints as the single-shot deleteAccount handler this flow
// replaces), run inside the caller's transaction.
func (r *Repo) HardDeleteStudio(ctx context.Context, tx pgx.Tx, studioID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM studios WHERE id = $1`, studioID)
	if err != nil {
		return fmt.Errorf("delete studio: %w", err)
	}
	r.evict(studioID)
	return nil
}
