package studios

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Plan struct {
	ID           uuid.UUID `json:"id"`
	StudioID     uuid.UUID `json:"studioId"`
	PlanName     string    `json:"planName"`
	PriceSGD     int       `json:"priceSgd"`
	BillingCycle string    `json:"billingCycle"`
	Features     []string  `json:"features"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (r *Repo) ListPlans(ctx context.Context, studioID uuid.UUID) ([]Plan, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, plan_name, price_sgd, billing_cycle, features, is_active, created_at, updated_at
		FROM plans
		WHERE studio_id = $1
		ORDER BY price_sgd ASC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list plans query: %w", err)
	}
	defer rows.Close()

	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.StudioID, &p.PlanName, &p.PriceSGD, &p.BillingCycle, &p.Features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list plans scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type CreatePlanInput struct {
	PlanName     string   `json:"planName"`
	PriceSGD     int      `json:"priceSgd"`
	BillingCycle string   `json:"billingCycle"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"isActive"`
}

func (r *Repo) CreatePlan(ctx context.Context, studioID uuid.UUID, in CreatePlanInput) (Plan, error) {
	if in.BillingCycle == "" {
		in.BillingCycle = "monthly"
	}
	if in.Features == nil {
		in.Features = []string{}
	}
	var p Plan
	err := r.pool.QueryRow(ctx, `
		INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, studio_id, plan_name, price_sgd, billing_cycle, features, is_active, created_at, updated_at
	`, studioID, in.PlanName, in.PriceSGD, in.BillingCycle, in.Features, in.IsActive).Scan(
		&p.ID, &p.StudioID, &p.PlanName, &p.PriceSGD, &p.BillingCycle, &p.Features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}
	return p, nil
}

func (r *Repo) DeletePlan(ctx context.Context, studioID, planID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM plans WHERE studio_id = $1 AND id = $2`, studioID, planID)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type UpdatePlanInput struct {
	PlanName     *string   `json:"planName"`
	PriceSGD     *int      `json:"priceSgd"`
	BillingCycle *string   `json:"billingCycle"`
	Features     *[]string `json:"features"`
	IsActive     *bool     `json:"isActive"`
}

func (r *Repo) UpdatePlan(ctx context.Context, studioID, planID uuid.UUID, in UpdatePlanInput) error {
	q := "UPDATE plans SET updated_at = now()"
	args := []any{studioID, planID}

	if in.PlanName != nil {
		args = append(args, *in.PlanName)
		q += fmt.Sprintf(", plan_name = $%d", len(args))
	}
	if in.BillingCycle != nil {
		args = append(args, *in.BillingCycle)
		q += fmt.Sprintf(", billing_cycle = $%d", len(args))
	}
	if in.PriceSGD != nil {
		args = append(args, *in.PriceSGD)
		q += fmt.Sprintf(", price_sgd = $%d", len(args))
	}
	if in.Features != nil {
		args = append(args, *in.Features)
		q += fmt.Sprintf(", features = $%d", len(args))
	}
	if in.IsActive != nil {
		args = append(args, *in.IsActive)
		q += fmt.Sprintf(", is_active = $%d", len(args))
	}

	q += " WHERE studio_id = $1 AND id = $2"

	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update plan exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
