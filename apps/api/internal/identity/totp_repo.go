package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrTOTPNotEnrolled means userID has never completed TOTP enrollment
// (no confirmed row in user_totp).
var ErrTOTPNotEnrolled = errors.New("totp not enrolled")

// SaveTOTPSecretPending upserts userID's TOTP secret in the
// not-yet-enabled state — step one of enrollment (task 2.2), before the
// confirmation code is checked. A second call before confirmation
// (e.g. the user re-scans the QR code) simply replaces the pending
// secret.
func (r *Repo) SaveTOTPSecretPending(ctx context.Context, userID uuid.UUID, secretCiphertext string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_totp (user_id, secret_ciphertext, enabled)
		VALUES ($1, $2, false)
		ON CONFLICT (user_id) DO UPDATE SET
			secret_ciphertext = EXCLUDED.secret_ciphertext,
			enabled = false,
			enrolled_at = NULL,
			last_used_step = NULL,
			updated_at = now()
	`, userID, secretCiphertext)
	if err != nil {
		return fmt.Errorf("save pending totp secret: %w", err)
	}
	return nil
}

// totpRecord is the row shape GetTOTP returns — used both to validate a
// login code (needs secret_ciphertext + last_used_step) and to answer
// "is this user enrolled" (needs enabled).
type totpRecord struct {
	SecretCiphertext string
	Enabled          bool
	LastUsedStep     *int64
}

// GetTOTP returns userID's stored TOTP record, or ErrTOTPNotEnrolled if
// they've never started enrollment at all (no row). A row with
// Enabled=false means enrollment was started but never confirmed.
func (r *Repo) GetTOTP(ctx context.Context, userID uuid.UUID) (*totpRecord, error) {
	var rec totpRecord
	err := r.pool.QueryRow(ctx,
		`SELECT secret_ciphertext, enabled, last_used_step FROM user_totp WHERE user_id = $1`, userID,
	).Scan(&rec.SecretCiphertext, &rec.Enabled, &rec.LastUsedStep)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTOTPNotEnrolled
		}
		return nil, fmt.Errorf("get totp: %w", err)
	}
	return &rec, nil
}

// IsTOTPEnabled is a cheap existence+enabled check, for login-flow
// branching (does this user need a code step at all) without needing
// the full record.
func (r *Repo) IsTOTPEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var enabled bool
	err := r.pool.QueryRow(ctx, `SELECT enabled FROM user_totp WHERE user_id = $1`, userID).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check totp enabled: %w", err)
	}
	return enabled, nil
}

// EnableTOTP confirms enrollment — called once the user proves they can
// generate a valid code from the pending secret (task 2.2).
func (r *Repo) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE user_totp SET enabled = true, enrolled_at = now(), updated_at = now() WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("enable totp: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTOTPNotEnrolled
	}
	return nil
}

// DisableTOTP removes userID's TOTP enrollment entirely (reset/lost-
// device admin reset, task 2.8) — they must re-enroll from scratch, same
// as a brand-new user. Also wipes recovery codes, since they're only
// meaningful alongside an enrolled secret.
func (r *Repo) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_totp WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete totp: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return tx.Commit(ctx)
}

// MarkTOTPStepUsed records step as the last-accepted code for userID —
// the replay guard: a later login attempt presenting a code matching
// this same (or an earlier) step must be rejected even though it's
// still within the validity window (task 2.3).
func (r *Repo) MarkTOTPStepUsed(ctx context.Context, userID uuid.UUID, step int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE user_totp SET last_used_step = $2, updated_at = now() WHERE user_id = $1`, userID, step)
	if err != nil {
		return fmt.Errorf("mark totp step used: %w", err)
	}
	return nil
}

// CreateRecoveryCodes replaces userID's recovery codes with freshly
// hashed ones (enrollment, or an admin reset — task 2.2/2.8). hashes
// must already be bcrypt hashes of normalizeRecoveryCode's output.
func (r *Repo) CreateRecoveryCodes(ctx context.Context, userID uuid.UUID, hashes []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear old recovery codes: %w", err)
	}
	for _, h := range hashes {
		if _, err := tx.Exec(ctx, `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, userID, h); err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ConsumeRecoveryCode checks normalizedCode against userID's unused
// recovery-code hashes; if one matches, marks it used (single-use —
// task 2.7) and returns true. Scans all unused hashes (there are at
// most 10) rather than a direct hash lookup, since bcrypt hashes are
// salted and can't be queried by equality.
func (r *Repo) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, normalizedCode string) (bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, code_hash FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, userID,
	)
	if err != nil {
		return false, fmt.Errorf("list unused recovery codes: %w", err)
	}
	type candidate struct {
		id   uuid.UUID
		hash string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.hash); err != nil {
			rows.Close()
			return false, fmt.Errorf("scan recovery code: %w", err)
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("list unused recovery codes: %w", err)
	}

	for _, c := range candidates {
		if VerifyPassword(c.hash, normalizedCode) {
			if _, err := r.pool.Exec(ctx, `UPDATE user_recovery_codes SET used_at = now() WHERE id = $1`, c.id); err != nil {
				return false, fmt.Errorf("mark recovery code used: %w", err)
			}
			return true, nil
		}
	}
	return false, nil
}

// CountUnusedRecoveryCodes is the "do they have any left" check behind
// the lost-phone admin-reset chain (task 2.8: reset is offered/required
// once this hits zero).
func (r *Repo) CountUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unused recovery codes: %w", err)
	}
	return count, nil
}
