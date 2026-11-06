package identity

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
)

// StudioTOTPEnrollmentStats is one studio's enrollment share — the
// backend data behind the enrollment-progress dashboard (platform-
// security-hardening task 2.10). Super_admin-only; identity can't
// import studios for its name (would cycle), so the caller joins
// StudioID against whatever studio list it already has.
type StudioTOTPEnrollmentStats struct {
	StudioID      uuid.UUID `json:"studioId"`
	TotalStaff    int       `json:"totalStaff"`
	EnrolledStaff int       `json:"enrolledStaff"`
}

// ListTOTPEnrollmentStats returns every studio's studio_staff enrollment
// counts, most-incomplete first (so an admin sees which studios still
// need to enroll, not the ones already done). super_admin/studio_admin
// are reported platform-wide instead of per-studio, since neither is
// studio-scoped the way studio_staff is.
func (r *Repo) ListTOTPEnrollmentStats(ctx context.Context) ([]StudioTOTPEnrollmentStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.studio_id, count(*) AS total, count(t.user_id) FILTER (WHERE t.enabled) AS enrolled
		FROM users u
		LEFT JOIN user_totp t ON t.user_id = u.id
		WHERE u.role = 'studio_staff' AND u.deleted_at IS NULL AND u.studio_id IS NOT NULL
		GROUP BY u.studio_id
		ORDER BY (count(t.user_id) FILTER (WHERE t.enabled))::float / count(*) ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list totp enrollment stats: %w", err)
	}
	defer rows.Close()
	var out []StudioTOTPEnrollmentStats
	for rows.Next() {
		var s StudioTOTPEnrollmentStats
		if err := rows.Scan(&s.StudioID, &s.TotalStaff, &s.EnrolledStaff); err != nil {
			return nil, fmt.Errorf("scan totp enrollment stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// platformRoleTOTPEnrollmentStats is super_admin/studio_admin's
// platform-wide (not per-studio) enrollment share.
type platformRoleTOTPEnrollmentStats struct {
	Role     Role `json:"role"`
	Total    int  `json:"total"`
	Enrolled int  `json:"enrolled"`
}

func (r *Repo) listPlatformRoleTOTPEnrollmentStats(ctx context.Context) ([]platformRoleTOTPEnrollmentStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.role, count(*) AS total, count(t.user_id) FILTER (WHERE t.enabled) AS enrolled
		FROM users u
		LEFT JOIN user_totp t ON t.user_id = u.id
		WHERE u.role IN ('super_admin', 'studio_admin') AND u.deleted_at IS NULL
		GROUP BY u.role
	`)
	if err != nil {
		return nil, fmt.Errorf("list platform role totp enrollment stats: %w", err)
	}
	defer rows.Close()
	var out []platformRoleTOTPEnrollmentStats
	for rows.Next() {
		var s platformRoleTOTPEnrollmentStats
		if err := rows.Scan(&s.Role, &s.Total, &s.Enrolled); err != nil {
			return nil, fmt.Errorf("scan platform role totp enrollment stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type totpEnrollmentStatsRes struct {
	ByStudio     []StudioTOTPEnrollmentStats       `json:"byStudio"`
	PlatformWide []platformRoleTOTPEnrollmentStats `json:"platformWide"`
}

// totpEnrollmentStats godoc
//
//	@Summary		2FA enrollment progress (super_admin)
//	@Description	Per-studio studio_staff enrollment share, plus platform-wide super_admin/studio_admin share — the data behind the enrollment-progress dashboard (task 2.10's frontend is not built in this change).
//	@Tags			Auth
//	@Security		CookieAuth
//	@Success		200	{object}	totpEnrollmentStatsRes
//	@Router			/api/v1/admin/2fa-enrollment-stats [get]
func (h *Handler) totpEnrollmentStats(w http.ResponseWriter, r *http.Request) {
	byStudio, err := h.repo.ListTOTPEnrollmentStats(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	platformWide, err := h.repo.listPlatformRoleTOTPEnrollmentStats(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, totpEnrollmentStatsRes{ByStudio: byStudio, PlatformWide: platformWide})
}
