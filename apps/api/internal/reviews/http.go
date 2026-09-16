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

// Create godoc
//
//	@Summary		Submit a review
//	@Description	Public endpoint for a visitor to submit a testimonial/review. No auth required.
//	@Tags			Reviews
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateReviewInput	true	"Review payload"
//	@Success		201		{object}	Review
//	@Failure		400		{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		500		{object}	httpx.ErrorResponse
//	@Router			/api/v1/reviews [post]
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

// ListAll godoc
//
//	@Summary		List all reviews
//	@Description	Public endpoint returning every submitted review. No auth required.
//	@Tags			Reviews
//	@Produce		json
//	@Success		200	{array}		Review
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/api/v1/reviews [get]
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
