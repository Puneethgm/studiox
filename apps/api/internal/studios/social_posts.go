package studios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/platform/httpx"
)

type SocialPost struct {
	ID                   uuid.UUID `json:"id"`
	StudioID             uuid.UUID `json:"studioId"`
	Campaign             string    `json:"campaign"`
	CampaignShareUrl     string    `json:"campaignShareUrl,omitempty"`
	Platform             string    `json:"platform"`
	Copy                 string    `json:"copy"`
	MediaURL             string    `json:"mediaUrl"`
	Status               string    `json:"status"` // draft, scheduled, published, failed
	DeliveryMode         string    `json:"deliveryMode,omitempty"`
	ExternalResourceName string    `json:"externalResourceName,omitempty"`
	ScheduledAt          time.Time `json:"scheduledAt"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (r *Repo) ListSocialPosts(ctx context.Context, studioID string) ([]SocialPost, error) {
	var rows pgx.Rows
	var err error
	if studioID == "global" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, studio_id, campaign, COALESCE(campaign_share_url, ''), platform, copy, media_url, status, delivery_mode, external_resource_name, scheduled_at, created_at, updated_at
			FROM social_posts
			ORDER BY CASE WHEN status = 'published' THEN 1 ELSE 0 END, scheduled_at DESC
		`)
	} else {
		sID, errParse := uuid.Parse(studioID)
		if errParse != nil {
			return nil, fmt.Errorf("invalid studio ID: %w", errParse)
		}
		rows, err = r.pool.Query(ctx, `
			SELECT id, studio_id, campaign, COALESCE(campaign_share_url, ''), platform, copy, media_url, status, delivery_mode, external_resource_name, scheduled_at, created_at, updated_at
			FROM social_posts
			WHERE studio_id = $1
			ORDER BY CASE WHEN status = 'published' THEN 1 ELSE 0 END, scheduled_at DESC
		`, sID)
	}
	if err != nil {
		return nil, fmt.Errorf("list social posts: %w", err)
	}
	defer rows.Close()

	posts := make([]SocialPost, 0)
	for rows.Next() {
		var p SocialPost
		if err := rows.Scan(&p.ID, &p.StudioID, &p.Campaign, &p.CampaignShareUrl, &p.Platform, &p.Copy, &p.MediaURL, &p.Status, &p.DeliveryMode, &p.ExternalResourceName, &p.ScheduledAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan social post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *Repo) CreateSocialPost(ctx context.Context, p *SocialPost) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO social_posts (studio_id, campaign, campaign_share_url, platform, copy, media_url, status, scheduled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`, p.StudioID, p.Campaign, p.CampaignShareUrl, p.Platform, p.Copy, p.MediaURL, p.Status, p.ScheduledAt)
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *Repo) UpdateSocialPost(ctx context.Context, studioID string, id uuid.UUID, p *SocialPost) error {
	if studioID == "global" {
		_, err := r.pool.Exec(ctx, `
			UPDATE social_posts
			SET campaign = $2, campaign_share_url = $3, platform = $4, copy = $5, media_url = $6, status = $7, scheduled_at = $8, updated_at = now()
			WHERE id = $1
		`, id, p.Campaign, p.CampaignShareUrl, p.Platform, p.Copy, p.MediaURL, p.Status, p.ScheduledAt)
		return err
	} else {
		sID, errParse := uuid.Parse(studioID)
		if errParse != nil {
			return fmt.Errorf("invalid studio ID: %w", errParse)
		}
		_, err := r.pool.Exec(ctx, `
			UPDATE social_posts
			SET campaign = $3, campaign_share_url = $4, platform = $5, copy = $6, media_url = $7, status = $8, scheduled_at = $9, updated_at = now()
			WHERE id = $1 AND studio_id = $2
		`, id, sID, p.Campaign, p.CampaignShareUrl, p.Platform, p.Copy, p.MediaURL, p.Status, p.ScheduledAt)
		return err
	}
}

func (r *Repo) DeleteSocialPost(ctx context.Context, studioID string, id uuid.UUID) error {
	var err error
	if studioID == "global" {
		_, err = r.pool.Exec(ctx, `DELETE FROM social_posts WHERE id = $1`, id)
	} else {
		sID, errParse := uuid.Parse(studioID)
		if errParse != nil {
			return fmt.Errorf("invalid studio ID: %w", errParse)
		}
		_, err = r.pool.Exec(ctx, `DELETE FROM social_posts WHERE id = $1 AND studio_id = $2`, id, sID)
	}
	return err
}

func (s *Service) ListSocialPosts(ctx context.Context, studioID string) ([]SocialPost, error) {
	return s.repo.ListSocialPosts(ctx, studioID)
}

func (s *Service) CreateSocialPost(ctx context.Context, p *SocialPost) error {
	if p.Status == "" {
		p.Status = "scheduled"
	}
	return s.repo.CreateSocialPost(ctx, p)
}

func (s *Service) UpdateSocialPost(ctx context.Context, studioID string, id uuid.UUID, p *SocialPost) error {
	if p.Status == "" {
		p.Status = "scheduled"
	}
	return s.repo.UpdateSocialPost(ctx, studioID, id, p)
}

func (s *Service) DeleteSocialPost(ctx context.Context, studioID string, id uuid.UUID) error {
	return s.repo.DeleteSocialPost(ctx, studioID, id)
}

func (h *Handler) ListSocialPosts(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "studioId")
	posts, err := h.svc.ListSocialPosts(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, posts)
}

func (h *Handler) CreateSocialPost(w http.ResponseWriter, r *http.Request) {
	studioIDStr := chi.URLParam(r, "studioId")
	if studioIDStr == "global" {
		httpx.WriteError(w, http.StatusBadRequest, "forbidden", "cannot create post on global scope")
		return
	}
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid studio ID")
		return
	}

	var req struct {
		Campaign         string    `json:"campaign"`
		CampaignShareUrl string    `json:"campaignShareUrl"`
		Platform         string    `json:"platform"`
		Copy             string    `json:"copy"`
		MediaURL         string    `json:"mediaUrl"`
		Status           string    `json:"status"`
		ScheduledAt      time.Time `json:"scheduledAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	p := SocialPost{
		StudioID:         studioID,
		Campaign:         req.Campaign,
		CampaignShareUrl: req.CampaignShareUrl,
		Platform:         req.Platform,
		Copy:             req.Copy,
		MediaURL:         req.MediaURL,
		Status:           req.Status,
		ScheduledAt:      req.ScheduledAt,
	}

	if err := h.svc.CreateSocialPost(r.Context(), &p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) UpdateSocialPost(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "studioId")
	postIDStr := chi.URLParam(r, "postId")
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid post ID")
		return
	}

	var req struct {
		Campaign         string    `json:"campaign"`
		CampaignShareUrl string    `json:"campaignShareUrl"`
		Platform         string    `json:"platform"`
		Copy             string    `json:"copy"`
		MediaURL         string    `json:"mediaUrl"`
		Status           string    `json:"status"`
		ScheduledAt      time.Time `json:"scheduledAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	p := SocialPost{
		Campaign:         req.Campaign,
		CampaignShareUrl: req.CampaignShareUrl,
		Platform:         req.Platform,
		Copy:             req.Copy,
		MediaURL:         req.MediaURL,
		Status:           req.Status,
		ScheduledAt:      req.ScheduledAt,
	}

	if err := h.svc.UpdateSocialPost(r.Context(), studioID, postID, &p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) DeleteSocialPost(w http.ResponseWriter, r *http.Request) {
	studioID := chi.URLParam(r, "studioId")
	postIDStr := chi.URLParam(r, "postId")
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid post ID")
		return
	}

	if err := h.svc.DeleteSocialPost(r.Context(), studioID, postID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
