package attendance

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	repo *Repo
}

func NewHandler(repo *Repo) *Handler {
	return &Handler{repo: repo}
}

const defaultPageSize = 50

// getAttendance godoc
//
//	@Summary		Get Glofox class attendance
//	@Description	Returns one page of the studio's Glofox attendance snapshot — each member's attended-class count within their current plan, refreshed every 15 minutes by a background worker. Read-only; sends nothing.
//	@Tags			Glofox
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			limit		query		int		false	"Page size (default 50)"
//	@Param			offset		query		int		false	"Row offset (default 0)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/glofox/attendance [get]
func (h *Handler) getAttendance(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	limit := defaultPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	rows, total, err := h.repo.List(r.Context(), studioID, limit, offset)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	threshold, err := h.repo.GetQualifyingThreshold(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	qualifying, err := h.repo.CountAtOrAbove(r.Context(), studioID, threshold)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	lastSyncedAt, err := h.repo.GetLastSyncedAt(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"members":             rows,
		"total":               total,
		"qualifyingThreshold": threshold,
		"qualifyingCount":     qualifying,
		"lastSyncedAt":        lastSyncedAt,
	})
}

// getThreshold godoc
//
//	@Summary		Get the Glofox attendance qualifying threshold
//	@Description	Returns how many classes a member must attend (within their current plan) to be flagged as qualifying on the Attendance page. Defaults to 5 until the studio sets its own.
//	@Tags			Glofox
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/glofox/attendance/threshold [get]
func (h *Handler) getThreshold(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	threshold, err := h.repo.GetQualifyingThreshold(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"qualifyingThreshold": threshold})
}

type putThresholdReq struct {
	QualifyingThreshold int `json:"qualifyingThreshold"`
}

// putThreshold godoc
//
//	@Summary		Set the Glofox attendance qualifying threshold
//	@Description	Sets how many classes a member must attend (within their current plan) to be flagged as qualifying. Takes effect from the worker's next poll.
//	@Tags			Glofox
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string			true	"Studio ID"
//	@Param			body		body		putThresholdReq	true	"New threshold"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/glofox/attendance/threshold [put]
func (h *Handler) putThreshold(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	var req putThresholdReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.QualifyingThreshold <= 0 {
		httpx.WriteValidationError(w, map[string]string{"qualifyingThreshold": "must be greater than 0"})
		return
	}
	if err := h.repo.SetQualifyingThreshold(r.Context(), studioID, req.QualifyingThreshold); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"qualifyingThreshold": req.QualifyingThreshold})
}

// AdminRoutes are mounted under /api/v1/studios/{studioId}.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/glofox/attendance", h.getAttendance)
	r.Get("/glofox/attendance/threshold", h.getThreshold)
	r.Put("/glofox/attendance/threshold", h.putThreshold)
}
