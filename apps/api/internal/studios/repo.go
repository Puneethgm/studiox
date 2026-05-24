package studios

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Pool exposes the underlying pool so the studios service can run a
// transactional create-studio-with-admin flow.
func (r *Repo) Pool() *pgxpool.Pool { return r.pool }

func (r *Repo) Create(ctx context.Context, tx pgx.Tx, s *Studio) error {
	row := tx.QueryRow(ctx, `
		INSERT INTO studios (slug, name, brand_color, logo_url, contact_email, active, gemini_api_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at, updated_at
	`, s.Slug, s.Name, s.BrandColor, s.LogoURL, s.ContactEmail, s.Active, s.GeminiAPIKey)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSlugTaken
		}
		return fmt.Errorf("insert studio: %w", err)
	}
	return nil
}

func (r *Repo) List(ctx context.Context) ([]Studio, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.slug, s.name, s.brand_color, s.logo_url, s.contact_email,
		       s.active, s.created_at, s.updated_at, s.availability_slots, s.availability_timezone, s.gemini_api_key,
		       COALESCE(c.cnt, 0), COALESCE(l.cnt, 0)
		FROM studios s
		LEFT JOIN (SELECT studio_id, COUNT(*) AS cnt FROM campaigns GROUP BY studio_id) c
		  ON c.studio_id = s.id
		LEFT JOIN (SELECT studio_id, COUNT(*) AS cnt FROM leads GROUP BY studio_id) l
		  ON l.studio_id = s.id
		ORDER BY s.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list studios: %w", err)
	}
	defer rows.Close()
	out := make([]Studio, 0)
	for rows.Next() {
		var s Studio
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.BrandColor, &s.LogoURL, &s.ContactEmail,
			&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.GeminiAPIKey, &s.CampaignCount, &s.LeadCount); err != nil {
			return nil, fmt.Errorf("scan studio: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Studio, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, slug, name, brand_color, logo_url, contact_email, active, created_at, updated_at,
		       availability_slots, availability_timezone, gemini_api_key
		FROM studios WHERE id = $1
	`, id)
	return scanStudio(row)
}

func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Studio, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, slug, name, brand_color, logo_url, contact_email, active, created_at, updated_at,
		       availability_slots, availability_timezone, gemini_api_key
		FROM studios WHERE slug = $1
	`, slug)
	return scanStudio(row)
}

// Update writes the editable fields. Slug is intentionally NOT updatable here
// (changing a slug breaks every shared public link). Add a deliberate "rename
// slug" flow when needed.
func (r *Repo) Update(ctx context.Context, id uuid.UUID, name, brandColor, logoURL, contactEmail string, active bool, availabilitySlots []AvailabilitySlot, availabilityTimezone string, geminiAPIKey string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET name = $2, brand_color = $3, logo_url = $4, contact_email = $5, active = $6,
		    availability_slots = $7, availability_timezone = $8, gemini_api_key = $9, updated_at = now()
		WHERE id = $1`,
		id, name, brandColor, logoURL, contactEmail, active, availabilitySlots, availabilityTimezone, geminiAPIKey)
	if err != nil {
		return fmt.Errorf("update studio: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}



func scanStudio(row pgx.Row) (*Studio, error) {
	var s Studio
	if err := row.Scan(&s.ID, &s.Slug, &s.Name, &s.BrandColor, &s.LogoURL, &s.ContactEmail,
		&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.GeminiAPIKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan studio: %w", err)
	}
	return &s, nil
}
