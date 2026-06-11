package reviews

import (
	"net/http"

	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	repo *Repo
}

func NewHandler(repo *Repo) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input CreateReviewInput
	if !httpx.DecodeJSON(w, r, &input) {
		return
	}

	if input.Name == "" || input.Rating < 1 || input.Rating > 5 || input.ReviewText == "" {
		httpx.WriteError(w, http.StatusBadRequest, "validation", "name, rating (1-5), and review_text are required")
		return
	}

	review, err := h.repo.Create(ctx, input)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to create review")
		return
	}

	httpx.JSON(w, http.StatusCreated, review)
}

func (h *Handler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	reviews, err := h.repo.ListAll(ctx)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to fetch reviews")
		return
	}

	if reviews == nil {
		reviews = []Review{}
	}

	httpx.JSON(w, http.StatusOK, reviews)
}
