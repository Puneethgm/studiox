package messaging

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/projectx/api/internal/platform/httpx"
)

// LLMStat is one row of the aggregated LLM usage report.
type LLMStat struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Count        int     `json:"count"`
	SuccessCount int     `json:"successCount"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	TokensIn     int     `json:"tokensIn"`
	TokensOut    int     `json:"tokensOut"`
	Date         string  `json:"date"` // YYYY-MM-DD
}

// LLMStudioStat represents aggregated usage for a single studio and model.
type LLMStudioStat struct {
	StudioID     *uuid.UUID `json:"studioId"`
	StudioName   string     `json:"studioName"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
	Count        int        `json:"count"`
	SuccessCount int        `json:"successCount"`
	AvgLatencyMs float64    `json:"avgLatencyMs"`
	TokensIn     int        `json:"tokensIn"`
	TokensOut    int        `json:"tokensOut"`
}

// GetLLMStats returns per-day, per-provider usage aggregates for the last 30 days,
// as well as studio-level usage aggregations.
// Accessible only to super-admins via /api/v1/admin/llm-stats.
func (h *Handler) GetLLMStats(w http.ResponseWriter, r *http.Request) {
	since := time.Now().AddDate(0, 0, -30)

	rows, err := h.svc.repo.pool.Query(r.Context(), `
		SELECT
			provider,
			model,
			COUNT(*)::int                                       AS count,
			SUM(CASE WHEN success THEN 1 ELSE 0 END)::int      AS success_count,
			COALESCE(AVG(latency_ms), 0)                       AS avg_latency_ms,
			COALESCE(SUM(tokens_in), 0)::int                   AS tokens_in,
			COALESCE(SUM(tokens_out), 0)::int                  AS tokens_out,
			DATE(created_at)                                    AS date
		FROM llm_usage_logs
		WHERE created_at >= $1
		GROUP BY provider, model, DATE(created_at)
		ORDER BY date DESC, count DESC
	`, since)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to query LLM stats")
		return
	}
	defer rows.Close()

	stats := make([]LLMStat, 0)
	for rows.Next() {
		var s LLMStat
		var date time.Time
		if err := rows.Scan(&s.Provider, &s.Model, &s.Count, &s.SuccessCount, &s.AvgLatencyMs, &s.TokensIn, &s.TokensOut, &date); err != nil {
			continue
		}
		s.Date = date.Format("2006-01-02")
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to read LLM stats")
		return
	}

	// Query studio-level stats
	studioRows, err := h.svc.repo.pool.Query(r.Context(), `
		SELECT
			l.studio_id,
			COALESCE(s.name, 'Platform / System')               AS studio_name,
			l.provider,
			l.model,
			COUNT(*)::int                                       AS count,
			SUM(CASE WHEN l.success THEN 1 ELSE 0 END)::int      AS success_count,
			COALESCE(AVG(l.latency_ms), 0)                       AS avg_latency_ms,
			COALESCE(SUM(l.tokens_in), 0)::int                   AS tokens_in,
			COALESCE(SUM(l.tokens_out), 0)::int                  AS tokens_out
		FROM llm_usage_logs l
		LEFT JOIN studios s ON s.id = l.studio_id
		WHERE l.created_at >= $1
		GROUP BY l.studio_id, s.name, l.provider, l.model
		ORDER BY count DESC
	`, since)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to query LLM studio stats")
		return
	}
	defer studioRows.Close()

	studioStats := make([]LLMStudioStat, 0)
	for studioRows.Next() {
		var ss LLMStudioStat
		if err := studioRows.Scan(&ss.StudioID, &ss.StudioName, &ss.Provider, &ss.Model, &ss.Count, &ss.SuccessCount, &ss.AvgLatencyMs, &ss.TokensIn, &ss.TokensOut); err != nil {
			continue
		}
		studioStats = append(studioStats, ss)
	}
	if err := studioRows.Err(); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to read LLM studio stats")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"stats":       stats,
		"studioStats": studioStats,
	})
}

// SuperAdminRoutes mounts routes that require super-admin privileges and have
// no studio-scope (global platform-level views).
func (h *Handler) SuperAdminRoutes(r chi.Router) {
	r.Get("/llm-stats", h.GetLLMStats)
}
