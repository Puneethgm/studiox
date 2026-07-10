package studios

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PlanFeatures struct {
	AllowedChannels  int  `json:"allowedChannels"`
	HasSocialPlanner bool `json:"hasSocialPlanner"`
	MaxAIReplies     int  `json:"maxAiReplies"`
}

// GetFeaturesForStudio fetches the active subscription features for a studio.
// This implements the business logic gating rules.
func GetFeaturesForStudio(ctx context.Context, repo *Repo, studioID uuid.UUID) PlanFeatures {
	var tier string
	// Check if the studio has an active subscription
	err := repo.pool.QueryRow(ctx, "SELECT plan_tier FROM studio_subscriptions WHERE studio_id = $1 AND status = 'active' LIMIT 1", studioID).Scan(&tier)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			tier = "trial" // default fallback
		} else {
			tier = "trial" // fallback on error for safety
		}
	}

	switch tier {
	case "enterprise":
		return PlanFeatures{AllowedChannels: 999, HasSocialPlanner: true, MaxAIReplies: 999999}
	case "pro":
		return PlanFeatures{AllowedChannels: 8, HasSocialPlanner: true, MaxAIReplies: 10000}
	case "growth":
		return PlanFeatures{AllowedChannels: 3, HasSocialPlanner: false, MaxAIReplies: 2000}
	default: // "trial"
		return PlanFeatures{AllowedChannels: 1, HasSocialPlanner: false, MaxAIReplies: 200}
	}
}
