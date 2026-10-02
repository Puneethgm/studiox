// Package attendance tracks how many classes each Glofox member has
// attended, refreshed on a timer (see Worker) and read by the Attendance
// page in the admin UI. It is a read-side cache over Glofox's bookings
// endpoint, not a source of truth — Glofox is.
package attendance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Row is one member's latest attendance snapshot for a studio.
type Row struct {
	GlofoxUserID    string     `json:"glofoxUserId"`
	Name            string     `json:"name"`
	Phone           string     `json:"phone"` // blank until the member crosses the qualifying threshold — see tickStudio
	Email           string     `json:"email"` // same
	PlanName        string     `json:"planName"`
	PlanLimit       int        `json:"planLimit"`     // 0 means unlimited/unknown — see glofox.Membership.BookedEvents
	PlanStart       *time.Time `json:"planStart"`     // nil if the member has no active plan on record
	PlanEnd         *time.Time `json:"planEnd"`       // nil if open-ended or no active plan
	ClassesAttended int        `json:"classesAttended"` // within [PlanStart, PlanEnd] when both are set; lifetime otherwise
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

const upsertRowSQL = `
	INSERT INTO glofox_attendance (studio_id, glofox_user_id, name, phone, email, plan_name, plan_limit, plan_start, plan_end, classes_attended, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
	ON CONFLICT (studio_id, glofox_user_id) DO UPDATE SET
		name             = EXCLUDED.name,
		phone            = CASE WHEN EXCLUDED.phone     <> '' THEN EXCLUDED.phone     ELSE glofox_attendance.phone     END,
		email            = CASE WHEN EXCLUDED.email     <> '' THEN EXCLUDED.email     ELSE glofox_attendance.email     END,
		plan_name        = CASE WHEN EXCLUDED.plan_name <> '' THEN EXCLUDED.plan_name ELSE glofox_attendance.plan_name END,
		plan_limit       = EXCLUDED.plan_limit,
		plan_start       = EXCLUDED.plan_start,
		plan_end         = EXCLUDED.plan_end,
		classes_attended = EXCLUDED.classes_attended,
		updated_at       = now()
`

// UpsertRow stores one member's row immediately — the worker calls this per
// member as it's computed, rather than batching the whole poll into one
// write at the end, so the Attendance page fills in progressively instead
// of staying empty until every member (hundreds of Glofox calls) has been
// processed, and a crash or timeout partway through a poll doesn't lose
// everything already computed.
func (r *Repo) UpsertRow(ctx context.Context, studioID uuid.UUID, row Row) error {
	_, err := r.pool.Exec(ctx, upsertRowSQL,
		studioID, row.GlofoxUserID, row.Name, row.Phone, row.Email, row.PlanName, row.PlanLimit, row.PlanStart, row.PlanEnd, row.ClassesAttended)
	if err != nil {
		return fmt.Errorf("upsert glofox_attendance row %s: %w", row.GlofoxUserID, err)
	}
	return nil
}

// List returns one page of tracked members for a studio, highest attendance
// first, plus the total row count (for the UI's pagination control).
func (r *Repo) List(ctx context.Context, studioID uuid.UUID, limit, offset int) ([]Row, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM glofox_attendance WHERE studio_id = $1`, studioID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count glofox_attendance: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT glofox_user_id, name, phone, email, plan_name, plan_limit, plan_start, plan_end, classes_attended, updated_at
		FROM glofox_attendance
		WHERE studio_id = $1
		ORDER BY classes_attended DESC, name ASC
		LIMIT $2 OFFSET $3
	`, studioID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list glofox_attendance: %w", err)
	}
	defer rows.Close()

	out := make([]Row, 0)
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.GlofoxUserID, &row.Name, &row.Phone, &row.Email, &row.PlanName, &row.PlanLimit, &row.PlanStart, &row.PlanEnd, &row.ClassesAttended, &row.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan glofox_attendance row: %w", err)
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// CountAtOrAbove returns how many of a studio's tracked members have
// classes_attended >= threshold — the total across the whole table, not
// just whatever page the UI happens to be viewing.
func (r *Repo) CountAtOrAbove(ctx context.Context, studioID uuid.UUID, threshold int) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM glofox_attendance WHERE studio_id = $1 AND classes_attended >= $2
	`, studioID, threshold).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count at or above threshold: %w", err)
	}
	return count, nil
}

// GetLastSyncedAt returns the most recent worker update for a studio,
// independent of pagination (List's own max(updated_at) would only reflect
// whichever page was fetched, not the whole table).
func (r *Repo) GetLastSyncedAt(ctx context.Context, studioID uuid.UUID) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT max(updated_at) FROM glofox_attendance WHERE studio_id = $1
	`, studioID).Scan(&t)
	if err != nil {
		return nil, fmt.Errorf("get last synced at: %w", err)
	}
	return t, nil
}

// defaultQualifyingThreshold is used for a studio that hasn't set its own
// value in glofox_attendance_settings yet.
const defaultQualifyingThreshold = 5

// GetQualifyingThreshold returns the studio's configured "classes attended"
// threshold — the cutoff at which the worker resolves full contact details
// and the UI flags a member as qualifying. Studio-configurable via the UI,
// not hardcoded.
func (r *Repo) GetQualifyingThreshold(ctx context.Context, studioID uuid.UUID) (int, error) {
	var threshold int
	err := r.pool.QueryRow(ctx, `
		SELECT qualifying_threshold FROM glofox_attendance_settings WHERE studio_id = $1
	`, studioID).Scan(&threshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultQualifyingThreshold, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get qualifying threshold: %w", err)
	}
	return threshold, nil
}

// SetQualifyingThreshold creates or updates the studio's threshold.
func (r *Repo) SetQualifyingThreshold(ctx context.Context, studioID uuid.UUID, threshold int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO glofox_attendance_settings (studio_id, qualifying_threshold, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (studio_id) DO UPDATE SET qualifying_threshold = EXCLUDED.qualifying_threshold, updated_at = now()
	`, studioID, threshold)
	if err != nil {
		return fmt.Errorf("set qualifying threshold: %w", err)
	}
	return nil
}
