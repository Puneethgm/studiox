package reviews

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Review struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Rating     int       `json:"rating"`
	ReviewText string    `json:"review_text"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateReviewInput struct {
	Name       string `json:"name" validate:"required,min=2,max=255"`
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
	ReviewText string `json:"review_text" validate:"required,min=10,max=5000"`
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, input CreateReviewInput) (*Review, error) {
	query := `
		INSERT INTO reviews (name, rating, review_text)
		VALUES ($1, $2, $3)
		RETURNING id, name, rating, review_text, created_at
	`

	var review Review
	err := r.pool.QueryRow(ctx, query, input.Name, input.Rating, input.ReviewText).
		Scan(&review.ID, &review.Name, &review.Rating, &review.ReviewText, &review.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	return &review, nil
}

func (r *Repo) ListAll(ctx context.Context) ([]Review, error) {
	query := `
		SELECT id, name, rating, review_text, created_at
		FROM reviews
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviews: %w", err)
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var review Review
		err := rows.Scan(&review.ID, &review.Name, &review.Rating, &review.ReviewText, &review.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan review: %w", err)
		}
		reviews = append(reviews, review)
	}

	return reviews, rows.Err()
}
