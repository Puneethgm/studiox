package leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ----- campaigns -----

func (r *Repo) CreateCampaign(ctx context.Context, c *Campaign) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO campaigns (studio_id, slug, name, description, fitness_plans, active, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at, updated_at
	`, c.StudioID, c.Slug, c.Name, c.Description, c.FitnessPlans, c.Active, c.CreatedBy)
	if err := row.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrSlugTaken
		}
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (r *Repo) ListCampaigns(ctx context.Context, studioID uuid.UUID) ([]Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
		       c.active, c.created_by, c.created_at, c.updated_at,
		       COALESCE(l.cnt, 0)
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		LEFT JOIN (SELECT campaign_id, COUNT(*) AS cnt FROM leads GROUP BY campaign_id) l
		  ON l.campaign_id = c.id
		WHERE c.studio_id = $1
		ORDER BY c.created_at DESC
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()
	out := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
			&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.LeadCount); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) GetCampaign(ctx context.Context, studioID, id uuid.UUID) (*Campaign, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
		       c.active, c.created_by, c.created_at, c.updated_at
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		WHERE c.studio_id = $1 AND c.id = $2
	`, studioID, id)
	var c Campaign
	if err := row.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
		&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	return &c, nil
}

// GetActiveCampaignByStudioAndSlug is the public-facing lookup.
func (r *Repo) GetActiveCampaignByStudioAndSlug(ctx context.Context, studioSlug, campaignSlug string) (*Campaign, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
		       c.active, c.created_by, c.created_at, c.updated_at
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		WHERE s.slug = $1 AND c.slug = $2 AND c.active AND s.active
	`, studioSlug, campaignSlug)
	var c Campaign
	if err := row.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
		&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("get public campaign: %w", err)
	}
	return &c, nil
}

func (r *Repo) SetCampaignActive(ctx context.Context, studioID, id uuid.UUID, active bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET active = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, active)
	if err != nil {
		return fmt.Errorf("set active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

// ----- leads -----

// CreateLeadWithOutbox writes the lead and a matching outbox row in a single
// transaction so we can never have a lead in DB without a queued export, or
// vice versa.
func (r *Repo) CreateLeadWithOutbox(ctx context.Context, l *Lead, destination string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, email, phone, fitness_plan,
		                   goals, source, referrer, user_agent, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, status, created_at, updated_at
	`, l.StudioID, l.CampaignID, l.Name, l.Email, l.Phone, l.FitnessPlan,
		l.Goals, l.Source, l.Referrer, l.UserAgent, ipText(l.IPAddress))
	if err := row.Scan(&l.ID, &l.Status, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return fmt.Errorf("insert lead: %w", err)
	}

	payload, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshal lead payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
		VALUES ('lead', $1, 'lead.created', $2, $3)
	`, l.ID, destination, payload); err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	// Also enqueue an autocontact job so the auto-contact worker can pick it up.
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
		VALUES ('lead', $1, 'lead.created', 'lead_autocontact', $2)
	`, l.ID, payload); err != nil {
		return fmt.Errorf("insert autocontact outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type ListLeadsFilter struct {
	CampaignID *uuid.UUID
	Status     *LeadStatus
	Limit      int
	Offset     int
}

func (r *Repo) ListLeads(ctx context.Context, studioID uuid.UUID, f ListLeadsFilter) ([]Lead, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	conds := []string{"l.studio_id = $1"}
	args := []any{studioID}
	if f.CampaignID != nil {
		args = append(args, *f.CampaignID)
		conds = append(conds, fmt.Sprintf("l.campaign_id = $%d", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conds = append(conds, fmt.Sprintf("l.status = $%d", len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads l WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	q := `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.notes, l.created_at, l.updated_at
		FROM leads l
		JOIN campaigns c ON c.id = l.campaign_id
		JOIN studios s ON s.id = l.studio_id
		WHERE ` + where + `
		ORDER BY l.created_at DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	out := make([]Lead, 0)
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID, &l.CampaignName, &l.CampaignSlug,
			&l.Name, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
			&l.Source, &l.Status, &l.Notes, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan lead: %w", err)
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

func (r *Repo) GetLead(ctx context.Context, studioID, id uuid.UUID) (*Lead, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.notes, l.created_at, l.updated_at
		FROM leads l
		JOIN campaigns c ON c.id = l.campaign_id
		JOIN studios s ON s.id = l.studio_id
		WHERE l.studio_id = $1 AND l.id = $2
	`, studioID, id)
	var l Lead
	if err := row.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID, &l.CampaignName, &l.CampaignSlug,
		&l.Name, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
		&l.Source, &l.Status, &l.Notes, &l.CreatedAt, &l.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLeadNotFound
		}
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return &l, nil
}

// LeadStats is a tiny aggregate used by the studio overview widgets and by
// the pipeline view's column counts. One round-trip, one tiny grouped query.
type LeadStats struct {
	Total    int                `json:"total"`
	ByStatus map[LeadStatus]int `json:"byStatus"`
}

func (r *Repo) Stats(ctx context.Context, studioID uuid.UUID) (*LeadStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM leads WHERE studio_id = $1 GROUP BY status
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	defer rows.Close()

	out := &LeadStats{ByStatus: make(map[LeadStatus]int, 5)}
	// Seed all five statuses so the response always has the same shape.
	for _, s := range []LeadStatus{StatusNew, StatusContacted, StatusTrialBooked, StatusMember, StatusDropped} {
		out.ByStatus[s] = 0
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}
		out.ByStatus[LeadStatus(status)] = count
		out.Total += count
	}
	return out, rows.Err()
}

func (r *Repo) UpdateLead(ctx context.Context, studioID, id uuid.UUID, status LeadStatus, notes string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads SET status = $3, notes = $4, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, string(status), notes)
	if err != nil {
		return fmt.Errorf("update lead: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

// UpdateStatus updates only the lead status (used by AI worker for auto-status updates)
func (r *Repo) UpdateStatus(ctx context.Context, studioID, id uuid.UUID, status LeadStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads SET status = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, string(status))
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

// MarkLeadContacted increments contact attempts, sets last_contacted_at, and marks status=contacted.
func (r *Repo) MarkLeadContacted(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE leads
		SET contact_attempts = contact_attempts + 1,
			last_contacted_at = now(),
			status = 'contacted',
			updated_at = now()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark lead contacted: %w", err)
	}
	return nil
}

// ----- outbox helpers -----

type OutboxItem struct {
	ID            int64
	AggregateID   uuid.UUID
	EventType     string
	Destination   string
	Payload       []byte
	Attempts      int
	NextAttemptAt time.Time
}

func (r *Repo) ClaimOutboxBatch(ctx context.Context, destination string, n int) ([]OutboxItem, error) {
	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT id FROM outbox
			WHERE status = 'pending'
			  AND destination = $1
			  AND next_attempt_at <= now()
			ORDER BY id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox o
		SET next_attempt_at = now() + INTERVAL '1 minute'
		FROM picked
		WHERE o.id = picked.id
		RETURNING o.id, o.aggregate_id, o.event_type, o.destination, o.payload, o.attempts, o.next_attempt_at
	`, destination, n)
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	defer rows.Close()

	out := make([]OutboxItem, 0)
	for rows.Next() {
		var it OutboxItem
		if err := rows.Scan(&it.ID, &it.AggregateID, &it.EventType, &it.Destination, &it.Payload, &it.Attempts, &it.NextAttemptAt); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *Repo) MarkOutboxSent(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox SET status = 'sent', sent_at = now(), last_error = ''
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}

func (r *Repo) MarkOutboxFailed(ctx context.Context, id int64, errMsg string, backoff time.Duration, dead bool) error {
	status := "pending"
	if dead {
		status = "dead"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE outbox
		SET attempts = attempts + 1,
		    next_attempt_at = now() + ($3 * INTERVAL '1 second'),
		    last_error = $2,
		    status = $4
		WHERE id = $1
	`, id, errMsg, backoff.Seconds(), status)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// ----- helpers -----

func ipText(ip *net.IP) any {
	if ip == nil {
		return nil
	}
	return ip.String()
}
