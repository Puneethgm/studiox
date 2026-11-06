package studios

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Offer is a studio's structured, time-limited promotion
// (ai-reply-flow-llm-design doc §4.4) — handed directly into the AI
// reply prompt as a fact, and checked against by the guards capability,
// instead of the model inferring or retrieving offer details.
type Offer struct {
	ID          uuid.UUID `json:"id"`
	StudioID    uuid.UUID `json:"studioId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Terms       string    `json:"terms"`
	PriceSGD    int       `json:"priceSgd"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (r *Repo) ListOffers(ctx context.Context, studioID uuid.UUID) ([]Offer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, name, description, terms, price_sgd, is_active, created_at, updated_at
		FROM offers
		WHERE studio_id = $1
		ORDER BY created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list offers query: %w", err)
	}
	defer rows.Close()

	var out []Offer
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.StudioID, &o.Name, &o.Description, &o.Terms, &o.PriceSGD, &o.IsActive, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list offers scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListActiveOffers is ListOffers filtered to is_active — the set actually
// handed into the AI reply prompt's data pack (an inactive offer must
// never be mentioned to a customer).
func (r *Repo) ListActiveOffers(ctx context.Context, studioID uuid.UUID) ([]Offer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, name, description, terms, price_sgd, is_active, created_at, updated_at
		FROM offers
		WHERE studio_id = $1 AND is_active = TRUE
		ORDER BY created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list active offers query: %w", err)
	}
	defer rows.Close()

	var out []Offer
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.StudioID, &o.Name, &o.Description, &o.Terms, &o.PriceSGD, &o.IsActive, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list active offers scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type CreateOfferInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Terms       string `json:"terms"`
	PriceSGD    int    `json:"priceSgd"`
	IsActive    bool   `json:"isActive"`
}

func (r *Repo) CreateOffer(ctx context.Context, studioID uuid.UUID, in CreateOfferInput) (Offer, error) {
	var o Offer
	err := r.pool.QueryRow(ctx, `
		INSERT INTO offers (studio_id, name, description, terms, price_sgd, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, studio_id, name, description, terms, price_sgd, is_active, created_at, updated_at
	`, studioID, in.Name, in.Description, in.Terms, in.PriceSGD, in.IsActive).Scan(
		&o.ID, &o.StudioID, &o.Name, &o.Description, &o.Terms, &o.PriceSGD, &o.IsActive, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return Offer{}, fmt.Errorf("create offer: %w", err)
	}
	return o, nil
}

type UpdateOfferInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Terms       *string `json:"terms"`
	PriceSGD    *int    `json:"priceSgd"`
	IsActive    *bool   `json:"isActive"`
}

func (r *Repo) UpdateOffer(ctx context.Context, studioID, offerID uuid.UUID, in UpdateOfferInput) error {
	q := "UPDATE offers SET updated_at = now()"
	args := []any{studioID, offerID}

	if in.Name != nil {
		args = append(args, *in.Name)
		q += fmt.Sprintf(", name = $%d", len(args))
	}
	if in.Description != nil {
		args = append(args, *in.Description)
		q += fmt.Sprintf(", description = $%d", len(args))
	}
	if in.Terms != nil {
		args = append(args, *in.Terms)
		q += fmt.Sprintf(", terms = $%d", len(args))
	}
	if in.PriceSGD != nil {
		args = append(args, *in.PriceSGD)
		q += fmt.Sprintf(", price_sgd = $%d", len(args))
	}
	if in.IsActive != nil {
		args = append(args, *in.IsActive)
		q += fmt.Sprintf(", is_active = $%d", len(args))
	}

	q += " WHERE studio_id = $1 AND id = $2"

	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update offer exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteOffer(ctx context.Context, studioID, offerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM offers WHERE studio_id = $1 AND id = $2`, studioID, offerID)
	if err != nil {
		return fmt.Errorf("delete offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
