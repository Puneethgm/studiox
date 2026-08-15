package leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/integrations/glofox"
)

type Repo struct {
	pool   *pgxpool.Pool
	glofox *glofox.Client
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// SetGlofoxClient wires up Glofox CRM sync for leads created/updated through
// this repo directly (the automated sheet-import and AI/automation status
// paths) — separate from the Service-level sync used by the manual "edit
// lead" HTTP flow. Nil-safe: sync is skipped entirely if never set (e.g. in
// tests, or when Glofox isn't configured).
func (r *Repo) SetGlofoxClient(gf *glofox.Client) { r.glofox = gf }

// syncLeadToGlofoxAsync fires the Glofox CRM sync in the background and never
// blocks or fails the caller — mirrors the fire-and-forget sync used by the
// manual lead-edit flow (Service.UpdateLead), but shared here so the
// automated paths (sheet import, AI/automation status changes) get the same
// behavior instead of silently skipping Glofox entirely.
func (r *Repo) syncLeadToGlofoxAsync(l *Lead, status LeadStatus) {
	if r.glofox == nil || l == nil {
		return
	}
	if status != StatusTrialBooked && status != StatusMember {
		return
	}
	gfStatus := glofox.GlofoxStatusTrial
	if status == StatusMember {
		gfStatus = glofox.GlofoxStatusMember
	}
	leadID := l.ID
	leadName := strings.TrimSpace(l.FirstName + " " + l.LastName)
	if leadName == "" {
		leadName = l.Name
	}
	leadEmail := l.Email
	leadPhone := l.Phone
	firstName := l.FirstName
	lastName := l.LastName
	if lastName == "" {
		// Glofox rejects the request outright without a last name — many
		// WhatsApp leads only ever give a first name.
		lastName = "-"
	}
	gender := l.Gender
	birthDate := ""
	if l.DateOfBirth != nil {
		birthDate = l.DateOfBirth.Format("2006-01-02")
	}
	go func() {
		gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := r.glofox.CreateLead(gCtx, glofox.CreateLeadInput{
			Email:         leadEmail,
			FirstName:     firstName,
			LastName:      lastName,
			Phone:         leadPhone,
			LeadStatus:    gfStatus,
			ContactSource: l.Source,
			Gender:        gender,
			BirthDate:     birthDate,
		})
		if err != nil {
			slog.Warn("Glofox | Lead sync failed — lead conversion not reflected in Glofox CRM",
				"component", "glofox",
				"lead_id", leadID,
				"lead_name", leadName,
				"lead_email", leadEmail,
				"new_status", string(status),
				"glofox_status", string(gfStatus),
				"error", err.Error(),
			)
		} else {
			slog.Info("Glofox | Lead synced to Glofox CRM",
				"component", "glofox",
				"lead_id", leadID,
				"lead_name", leadName,
				"lead_email", leadEmail,
				"new_status", string(status),
				"glofox_status", string(gfStatus),
				"glofox_id", out.Entity.ID,
			)
		}
	}()
}

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

func (r *Repo) ListCampaigns(ctx context.Context, studioID uuid.UUID, limit, offset int) ([]Campaign, int, error) {
	var total int
	var err error
	if studioID == uuid.Nil {
		err = r.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM campaigns
		`).Scan(&total)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM campaigns WHERE studio_id = $1
		`, studioID).Scan(&total)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("count campaigns: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	var rows pgx.Rows
	if studioID == uuid.Nil {
		rows, err = r.pool.Query(ctx, `
			SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
			       c.active, c.created_by, c.created_at, c.updated_at,
			       COALESCE(l.cnt, 0)
			FROM campaigns c
			JOIN studios s ON s.id = c.studio_id
			LEFT JOIN (SELECT campaign_id, COUNT(*) AS cnt FROM leads GROUP BY campaign_id) l
			  ON l.campaign_id = c.id
			ORDER BY c.created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
			       c.active, c.created_by, c.created_at, c.updated_at,
			       COALESCE(l.cnt, 0)
			FROM campaigns c
			JOIN studios s ON s.id = c.studio_id
			LEFT JOIN (SELECT campaign_id, COUNT(*) AS cnt FROM leads GROUP BY campaign_id) l
			  ON l.campaign_id = c.id
			WHERE c.studio_id = $1
			ORDER BY c.created_at DESC
			LIMIT $2 OFFSET $3
		`, studioID, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()
	out := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
			&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.LeadCount); err != nil {
			return nil, 0, fmt.Errorf("scan campaign: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *Repo) GetCampaign(ctx context.Context, studioID, id uuid.UUID) (*Campaign, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
		       c.active, c.created_by, c.created_at, c.updated_at,
		       COALESCE(l.cnt, 0)
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		LEFT JOIN (SELECT campaign_id, COUNT(*) AS cnt FROM leads GROUP BY campaign_id) l
		  ON l.campaign_id = c.id
		WHERE c.studio_id = $1 AND c.id = $2
	`, studioID, id)
	var c Campaign
	if err := row.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
		&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.LeadCount); err != nil {
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

func (r *Repo) UpdateCampaignFitnessPlans(ctx context.Context, studioID, id uuid.UUID, fitnessPlans []string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE campaigns
		SET fitness_plans = $3,
		    updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, fitnessPlans)
	if err != nil {
		return fmt.Errorf("update campaign fitness plans: %w", err)
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
func (r *Repo) CreateLeadWithOutbox(ctx context.Context, l *Lead, destination string, skipAutoContact bool) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if l.Status == "" {
		l.Status = StatusNew
	}

	if l.FirstName == "" && l.LastName == "" && l.Name != "" {
		parts := strings.SplitN(l.Name, " ", 2)
		l.FirstName = parts[0]
		if len(parts) > 1 {
			l.LastName = parts[1]
		}
	} else if l.Name == "" {
		l.Name = strings.TrimSpace(l.FirstName + " " + l.LastName)
	}

	if l.AutoContactStage == "" {
		l.AutoContactStage = "initial"
	}
	if l.Currency == "" {
		l.Currency = "SGD"
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO leads (studio_id, campaign_id, name, first_name, last_name, email, phone, fitness_plan,
		                   goals, source, status, currency, notes, contact_made, hot_lead, trial_purchased, auto_contact_stage,
		                   assigned_to, trial_attended, member_sold, monthly_fee, offer, further_notes, referrer, user_agent, ip_address)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
		RETURNING id, status, contact_attempts, last_contacted_at, contact_made, hot_lead, trial_purchased, auto_contact_stage,
		          assigned_to, trial_attended, member_sold, monthly_fee, currency, offer, further_notes, created_at, updated_at
	`, l.StudioID, l.CampaignID, l.Name, l.FirstName, l.LastName, l.Email, l.Phone, l.FitnessPlan,
		l.Goals, l.Source, l.Status, l.Currency, l.Notes, l.ContactMade, l.HotLead, l.TrialPurchased, l.AutoContactStage,
		l.AssignedTo, l.TrialAttended, l.MemberSold, l.MonthlyFee, l.Offer, l.FurtherNotes, l.Referrer, l.UserAgent, ipText(l.IPAddress))
	if err := row.Scan(&l.ID, &l.Status, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage,
		&l.AssignedTo, &l.TrialAttended, &l.MemberSold, &l.MonthlyFee, &l.Currency, &l.Offer, &l.FurtherNotes, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return fmt.Errorf("insert lead: %w", err)
	}

	type leadPayload struct {
		ID           string `json:"id"`
		StudioID     string `json:"studioId"`
		CampaignID   string `json:"campaignId"`
		Name         string `json:"name"`
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		FitnessPlan  string `json:"fitnessPlan"`
		Goals        string `json:"goals"`
		Source       string `json:"source"`
		Status       string `json:"status"`
		StudioName   string `json:"studioName"`
		CampaignName string `json:"campaignName"`
		StudioSlug   string `json:"studioSlug"`
		CampaignSlug string `json:"campaignSlug"`
	}
	payload, err := json.Marshal(leadPayload{
		ID: l.ID.String(), StudioID: l.StudioID.String(), CampaignID: l.CampaignID.String(),
		Name: l.Name, FirstName: l.FirstName, LastName: l.LastName,
		Email: l.Email, Phone: l.Phone, FitnessPlan: l.FitnessPlan,
		Goals: l.Goals, Source: l.Source, Status: string(l.Status),
		StudioName: l.StudioName, CampaignName: l.CampaignName,
		StudioSlug: l.StudioSlug, CampaignSlug: l.CampaignSlug,
	})
	if err != nil {
		return fmt.Errorf("marshal lead payload: %w", err)
	}
	if !skipAutoContact {
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
			VALUES ('lead', $1, 'lead.created', $2, $3)
		`, l.ID, destination, string(payload)); err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}

		// Also enqueue an autocontact job so the auto-contact worker can pick
		// it up — unless destination is already "lead_autocontact", in which
		// case the insert above already is that job; inserting it again would
		// duplicate the greeting and every scheduled follow-up.
		if destination != "lead_autocontact" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
				VALUES ('lead', $1, 'lead.created', 'lead_autocontact', $2)
			`, l.ID, string(payload)); err != nil {
				return fmt.Errorf("insert autocontact outbox: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Sync to Glofox CRM when the lead is created already at trial/member
	// status (e.g. a sheet-imported lead with Trial Purchased = YES) — this
	// path skips the AI worker's status-change flow entirely, so without
	// this it would never reach Glofox.
	r.syncLeadToGlofoxAsync(l, l.Status)

	return nil
}

type ListLeadsFilter struct {
	CampaignID     *uuid.UUID
	Status         *LeadStatus
	Statuses       []LeadStatus
	MaxAttempts    *int
	StartDate      string
	EndDate        string
	DurationDays   int
	Source         string
	HotLead        *bool
	ContactMade    *bool
	TrialPurchased *bool
	Limit          int
	Offset         int
	Search         string
}

func (r *Repo) ListLeads(ctx context.Context, studioID uuid.UUID, f ListLeadsFilter) ([]Lead, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	conds := []string{}
	args := []any{}
	if studioID != uuid.Nil {
		conds = append(conds, "l.studio_id = $1")
		args = append(args, studioID)
	}
	if f.CampaignID != nil {
		args = append(args, *f.CampaignID)
		conds = append(conds, fmt.Sprintf("l.campaign_id = $%d", len(args)))
	}
	if len(f.Statuses) > 0 {
		placeholders := make([]string, len(f.Statuses))
		for i, s := range f.Statuses {
			args = append(args, string(s))
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		conds = append(conds, fmt.Sprintf("l.status IN (%s)", strings.Join(placeholders, ",")))
	} else if f.Status != nil {
		args = append(args, string(*f.Status))
		conds = append(conds, fmt.Sprintf("l.status = $%d", len(args)))
	}
	if f.MaxAttempts != nil {
		args = append(args, *f.MaxAttempts)
		conds = append(conds, fmt.Sprintf("l.contact_attempts < $%d", len(args)))
	}
	if f.Source != "" {
		args = append(args, f.Source)
		conds = append(conds, fmt.Sprintf("l.source = $%d", len(args)))
	}

	var dateFilter string
	if f.StartDate != "" && f.EndDate != "" {
		dateFilter = fmt.Sprintf("l.created_at >= '%s'::timestamp AND l.created_at <= '%s 23:59:59'::timestamp", sanitizeDate(f.StartDate), sanitizeDate(f.EndDate))
	} else if f.DurationDays > 0 {
		if f.DurationDays == 1 {
			dateFilter = "l.created_at >= CURRENT_DATE - INTERVAL '1 day'"
		} else {
			dateFilter = fmt.Sprintf("l.created_at >= CURRENT_DATE - INTERVAL '%d days'", f.DurationDays)
		}
	}
	if dateFilter != "" {
		conds = append(conds, dateFilter)
	}

	if f.HotLead != nil {
		args = append(args, *f.HotLead)
		conds = append(conds, fmt.Sprintf("l.hot_lead = $%d", len(args)))
	}
	if f.ContactMade != nil {
		args = append(args, *f.ContactMade)
		conds = append(conds, fmt.Sprintf("l.contact_made = $%d", len(args)))
	}
	if f.TrialPurchased != nil {
		args = append(args, *f.TrialPurchased)
		conds = append(conds, fmt.Sprintf("l.trial_purchased = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		conds = append(conds, fmt.Sprintf("(LOWER(l.name) LIKE $%d OR LOWER(COALESCE(l.first_name, '')) LIKE $%d OR LOWER(COALESCE(l.last_name, '')) LIKE $%d OR LOWER(l.email) LIKE $%d OR l.phone LIKE $%d)", len(args), len(args), len(args), len(args), len(args)))
	}
	where := "1=1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads l WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count leads: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	q := `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, COALESCE(l.first_name, ''), COALESCE(l.last_name, ''), l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.currency, l.notes, l.contact_attempts, l.last_contacted_at, l.contact_made, l.hot_lead, l.trial_purchased, l.auto_contact_stage,
		       COALESCE(l.assigned_to, ''), l.trial_attended, l.member_sold, l.monthly_fee, COALESCE(l.offer, ''), COALESCE(l.further_notes, ''),
		       l.dnd_enabled, l.created_at, l.updated_at
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
			&l.Name, &l.FirstName, &l.LastName, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
			&l.Source, &l.Status, &l.Currency, &l.Notes, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage,
			&l.AssignedTo, &l.TrialAttended, &l.MemberSold, &l.MonthlyFee, &l.Offer, &l.FurtherNotes, &l.DNDEnabled, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan lead: %w", err)
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

func (r *Repo) GetLead(ctx context.Context, studioID, id uuid.UUID) (*Lead, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, COALESCE(l.first_name, ''), COALESCE(l.last_name, ''), l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.currency, l.notes, l.contact_attempts, l.last_contacted_at, l.contact_made, l.hot_lead, l.trial_purchased, l.auto_contact_stage,
		       COALESCE(l.assigned_to, ''), l.trial_attended, l.member_sold, l.monthly_fee, COALESCE(l.offer, ''), COALESCE(l.further_notes, ''),
		       l.dnd_enabled, l.created_at, l.updated_at, COALESCE(l.gender, ''), l.date_of_birth
		FROM leads l
		JOIN campaigns c ON c.id = l.campaign_id
		JOIN studios s ON s.id = l.studio_id
		WHERE l.studio_id = $1 AND l.id = $2
	`, studioID, id)
	var l Lead
	if err := row.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID, &l.CampaignName, &l.CampaignSlug,
		&l.Name, &l.FirstName, &l.LastName, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
		&l.Source, &l.Status, &l.Currency, &l.Notes, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage,
		&l.AssignedTo, &l.TrialAttended, &l.MemberSold, &l.MonthlyFee, &l.Offer, &l.FurtherNotes, &l.DNDEnabled, &l.CreatedAt, &l.UpdatedAt, &l.Gender, &l.DateOfBirth); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLeadNotFound
		}
		return nil, fmt.Errorf("get lead: %w", err)
	}
	return &l, nil
}

func (r *Repo) GetLeadTx(ctx context.Context, tx pgx.Tx, studioID, id uuid.UUID) (*Lead, error) {
	row := tx.QueryRow(ctx, `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, COALESCE(l.first_name, ''), COALESCE(l.last_name, ''), l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.currency, l.notes, l.contact_attempts, l.last_contacted_at, l.contact_made, l.hot_lead, l.trial_purchased, l.auto_contact_stage,
		       COALESCE(l.assigned_to, ''), l.trial_attended, l.member_sold, l.monthly_fee, COALESCE(l.offer, ''), COALESCE(l.further_notes, ''),
		       l.dnd_enabled, l.created_at, l.updated_at, COALESCE(l.gender, ''), l.date_of_birth
		FROM leads l
		JOIN campaigns c ON c.id = l.campaign_id
		JOIN studios s ON s.id = l.studio_id
		WHERE l.studio_id = $1 AND l.id = $2
	`, studioID, id)
	var l Lead
	if err := row.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID, &l.CampaignName, &l.CampaignSlug,
		&l.Name, &l.FirstName, &l.LastName, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
		&l.Source, &l.Status, &l.Currency, &l.Notes, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage,
		&l.AssignedTo, &l.TrialAttended, &l.MemberSold, &l.MonthlyFee, &l.Offer, &l.FurtherNotes, &l.DNDEnabled, &l.CreatedAt, &l.UpdatedAt, &l.Gender, &l.DateOfBirth); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLeadNotFound
		}
		return nil, fmt.Errorf("get lead tx: %w", err)
	}
	return &l, nil
}

// GetLeadPublic is a minimal, auth-free lookup for the pre-payment trial
// details page — returns only what's safe to expose without a session (no
// notes, no assigned staff, no contact history).
func (r *Repo) GetLeadPublic(ctx context.Context, id uuid.UUID) (*Lead, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id,
		       l.name, COALESCE(l.first_name,''), COALESCE(l.last_name,''), l.phone,
		       l.status, l.trial_purchased, COALESCE(l.gender,''), l.date_of_birth
		FROM leads l
		JOIN studios s ON s.id = l.studio_id
		WHERE l.id = $1
	`, id)
	var l Lead
	if err := row.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID,
		&l.Name, &l.FirstName, &l.LastName, &l.Phone,
		&l.Status, &l.TrialPurchased, &l.Gender, &l.DateOfBirth); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLeadNotFound
		}
		return nil, fmt.Errorf("get lead public: %w", err)
	}
	return &l, nil
}

// SaveTrialCheckoutDetails saves the full name/gender/date-of-birth collected
// on the pre-payment trial details page, right before Stripe checkout is
// created. Public-facing (no studio_id scoping needed — the lead id itself,
// a UUID, is the only thing an outsider could possibly have).
func (r *Repo) SaveTrialCheckoutDetails(ctx context.Context, id uuid.UUID, fullName, gender string, dob *time.Time) error {
	fullName = strings.TrimSpace(fullName)
	parts := strings.SplitN(fullName, " ", 2)
	firstName := parts[0]
	lastName := ""
	if len(parts) > 1 {
		lastName = parts[1]
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads
		SET name = $2, first_name = $3, last_name = $4, gender = $5, date_of_birth = $6, updated_at = now()
		WHERE id = $1
	`, id, fullName, firstName, lastName, gender, dob)
	if err != nil {
		return fmt.Errorf("save trial checkout details: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

// LeadStats is a tiny aggregate used by the studio overview widgets and by
// the pipeline view's column counts. One round-trip, one tiny grouped query.
type LeadStats struct {
	Total    int                `json:"total"`
	ByStatus map[LeadStatus]int `json:"byStatus"`
}

func (r *Repo) Stats(ctx context.Context, studioID uuid.UUID) (*LeadStats, error) {
	var rows pgx.Rows
	var err error
	if studioID == uuid.Nil {
		rows, err = r.pool.Query(ctx, `
			SELECT status, COUNT(*) FROM leads GROUP BY status
		`)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT status, COUNT(*) FROM leads WHERE studio_id = $1 GROUP BY status
		`, studioID)
	}
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	defer rows.Close()

	out := &LeadStats{ByStatus: make(map[LeadStatus]int, 6)}
	// Seed all six statuses so the response always has the same shape.
	for _, s := range []LeadStatus{StatusNew, StatusContacted, StatusTrialBooked, StatusMember, StatusDropped, StatusPaused} {
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

func (r *Repo) UpdateLead(ctx context.Context, studioID, id uuid.UUID, status LeadStatus, currency string, notes string, contactMade, hotLead, trialPurchased bool, firstName, lastName, fitnessPlan, assignedTo string, trialAttended, memberSold bool, monthlyFee float64, offer, furtherNotes string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldStatus, oldNotes, oldFirstName, oldLastName, oldFitnessPlan, oldAssignedTo, oldOffer, oldFurtherNotes, oldCurrency string
	var oldContactMade, oldHotLead, oldTrialPurchased, oldTrialAttended, oldMemberSold bool
	var oldMonthlyFee float64
	err = tx.QueryRow(ctx, `
		SELECT status, COALESCE(notes, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(fitness_plan, ''),
		       contact_made, hot_lead, trial_purchased,
		       COALESCE(assigned_to, ''), trial_attended, member_sold, monthly_fee, currency,
		       COALESCE(offer, ''), COALESCE(further_notes, '')
		FROM leads WHERE studio_id = $1 AND id = $2
	`, studioID, id).Scan(&oldStatus, &oldNotes, &oldFirstName, &oldLastName, &oldFitnessPlan,
		&oldContactMade, &oldHotLead, &oldTrialPurchased,
		&oldAssignedTo, &oldTrialAttended, &oldMemberSold, &oldMonthlyFee, &oldCurrency,
		&oldOffer, &oldFurtherNotes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLeadNotFound
		}
		return fmt.Errorf("select old details: %w", err)
	}

	if currency == "" {
		currency = "SGD"
	}

	name := strings.TrimSpace(firstName + " " + lastName)
	tag, err := tx.Exec(ctx, `
		UPDATE leads 
		SET status = $3, notes = $4, contact_made = $5, hot_lead = $6, trial_purchased = $7,
		    first_name = $8, last_name = $9, fitness_plan = $10, name = $11,
		    assigned_to = $12, trial_attended = $13, member_sold = $14, monthly_fee = $15,
		    currency = $16, offer = $17, further_notes = $18, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, string(status), notes, contactMade, hotLead, trialPurchased, firstName, lastName, fitnessPlan, name,
		assignedTo, trialAttended, memberSold, monthlyFee, currency, offer, furtherNotes)
	if err != nil {
		return fmt.Errorf("update lead: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}

	changed := string(status) != oldStatus || notes != oldNotes || firstName != oldFirstName || lastName != oldLastName || fitnessPlan != oldFitnessPlan ||
		contactMade != oldContactMade || hotLead != oldHotLead || trialPurchased != oldTrialPurchased ||
		assignedTo != oldAssignedTo || trialAttended != oldTrialAttended || memberSold != oldMemberSold ||
		monthlyFee != oldMonthlyFee || currency != oldCurrency || offer != oldOffer || furtherNotes != oldFurtherNotes

	if changed {
		l, err := r.GetLeadTx(ctx, tx, studioID, id)
		if err != nil {
			return fmt.Errorf("get updated lead: %w", err)
		}
		payload, err := json.Marshal(l)
		if err != nil {
			return fmt.Errorf("marshal lead: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
			VALUES ('lead', $1, 'lead.updated', 'google_sheets', $2)
		`, l.ID, string(payload))
		if err != nil {
			return fmt.Errorf("enqueue update outbox: %w", err)
		}
	}

	if trialAttended && !oldTrialAttended {
		var convID uuid.UUID
		err = tx.QueryRow(ctx, `
			SELECT id FROM conversations
			WHERE studio_id = $1 AND lead_id = $2
			ORDER BY updated_at DESC
			LIMIT 1
		`, studioID, id).Scan(&convID)
		if err == nil {
			msgBody := "Hi {{contact.first_name}}, we hope you're enjoying your trial! Are you ready to take the next step and become a member? Please select an option:\n1. Yes, I am ready!\n2. Not right now"
			_, err = tx.Exec(ctx, `
				INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
				                           source_kind, source_ref, scheduled_for, next_attempt_at)
				VALUES ($1, $2, $3, '[]'::jsonb, 'automation', $4, now(), now())
			`, studioID, convID, msgBody, fmt.Sprintf("lead:%s:trial_followup", id.String()))
			if err != nil {
				return fmt.Errorf("enqueue trial followup: %w", err)
			}

			// Enqueue future reminders for Phase 5
			delays := []time.Duration{24 * time.Hour, 72 * time.Hour}
			for i, d := range delays {
				body := "Hi {{contact.first_name}}, just checking in! Are you still interested in becoming a member at {{studio.name}}? Let us know if you have any questions!"
				if i == 1 {
					body = "Hi {{contact.first_name}}, we'd love to have you back at {{studio.name}}! Our membership spots are filling up fast. Ready to join us?"
				}
				_, err = tx.Exec(ctx, `
					INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
					                           source_kind, source_ref, scheduled_for, next_attempt_at)
					VALUES ($1, $2, $3, '[]'::jsonb, 'automation', $4, now() + ($5 * INTERVAL '1 second'), now() + ($5 * INTERVAL '1 second'))
				`, studioID, convID, body, fmt.Sprintf("lead:%s:trial_reminder:%d", id.String(), i+1), d.Seconds())
				if err != nil {
					return fmt.Errorf("enqueue trial reminder %d: %w", i, err)
				}
			}
		}
	}

	// Cancel any pending automated follow-ups if the lead became a member
	if string(status) == "member" && oldStatus != "member" {
		_, _ = tx.Exec(ctx, `
			DELETE FROM outbound_jobs
			WHERE studio_id = $1 AND conversation_id IN (
				SELECT id FROM conversations WHERE lead_id = $2
			) AND source_kind = 'automation' AND status = 'pending'
		`, studioID, id)
	}

	return tx.Commit(ctx)
}

// UpdateStatus updates only the lead status (used by AI worker for auto-status updates)
func (r *Repo) UpdateStatus(ctx context.Context, studioID, id uuid.UUID, status LeadStatus) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM leads WHERE studio_id = $1 AND id = $2`, studioID, id).Scan(&oldStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLeadNotFound
		}
		return fmt.Errorf("select old status: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE leads SET status = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, string(status))
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}

	var updatedLead *Lead
	if string(status) != oldStatus {
		l, err := r.GetLeadTx(ctx, tx, studioID, id)
		if err == nil {
			updatedLead = l
			payload, err := json.Marshal(l)
			if err == nil {
				_, _ = tx.Exec(ctx, `
					INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
					VALUES ('lead', $1, 'lead.updated', 'google_sheets', $2)
				`, l.ID, string(payload))
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Sync to Glofox CRM when the status transition lands on trial/member —
	// covers status changes made by the AI worker or automation, which
	// (unlike the manual "edit lead" HTTP flow) don't otherwise sync.
	if string(status) != oldStatus {
		r.syncLeadToGlofoxAsync(updatedLead, status)
	}

	return nil
}

// SetDNDEnabled toggles Do Not Disturb for a lead — silencing all automated
// messaging without touching pipeline status. Returns ErrLeadNotFound if the
// lead doesn't exist for this studio.
func (r *Repo) SetDNDEnabled(ctx context.Context, studioID, id uuid.UUID, enabled bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads SET dnd_enabled = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, enabled)
	if err != nil {
		return fmt.Errorf("set dnd enabled: %w", err)
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

// ----- studio sheets settings -----

func (r *Repo) GetSheetsSettings(ctx context.Context, studioID uuid.UUID) (*StudioSheetsSettings, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, spreadsheet_id, tab_name, active, created_at, updated_at
		FROM studio_sheets_settings
		WHERE studio_id = $1
	`, studioID)
	var s StudioSheetsSettings
	if err := row.Scan(&s.ID, &s.StudioID, &s.SpreadsheetID, &s.TabName, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil, nil if no settings exist yet
		}
		return nil, fmt.Errorf("get sheets settings: %w", err)
	}
	return &s, nil
}

// GetOldestActiveCampaign resolves the "default" campaign for a studio when
// importing leads from a source (like a Google Sheet) that doesn't specify one.
func (r *Repo) GetOldestActiveCampaign(ctx context.Context, studioID uuid.UUID) (*Campaign, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, slug, name, description, fitness_plans, active, created_by, created_at, updated_at
		FROM campaigns
		WHERE studio_id = $1 AND active = true
		ORDER BY created_at ASC
		LIMIT 1
	`, studioID)
	var c Campaign
	if err := row.Scan(&c.ID, &c.StudioID, &c.Slug, &c.Name, &c.Description, &c.FitnessPlans,
		&c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("get oldest active campaign: %w", err)
	}
	return &c, nil
}

// GetOldestActiveCampaignByStudioSlug is the same "default campaign" lookup
// as GetOldestActiveCampaign, but resolved directly from a public studio
// slug — used by the studio-wide (no-lead-attached) trial signup link.
func (r *Repo) GetOldestActiveCampaignByStudioSlug(ctx context.Context, studioSlug string) (*Campaign, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT c.id, c.studio_id, s.slug, s.name, c.slug, c.name, c.description, c.fitness_plans,
		       c.active, c.created_by, c.created_at, c.updated_at
		FROM campaigns c
		JOIN studios s ON s.id = c.studio_id
		WHERE s.slug = $1 AND c.active = true AND s.active = true
		ORDER BY c.created_at ASC
		LIMIT 1
	`, studioSlug)
	var c Campaign
	if err := row.Scan(&c.ID, &c.StudioID, &c.StudioSlug, &c.StudioName, &c.Slug, &c.Name, &c.Description,
		&c.FitnessPlans, &c.Active, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("get oldest active campaign by studio slug: %w", err)
	}
	return &c, nil
}

func (r *Repo) SaveSheetsSettings(ctx context.Context, s *StudioSheetsSettings) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO studio_sheets_settings (studio_id, spreadsheet_id, tab_name, active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (studio_id) DO UPDATE
		SET spreadsheet_id = EXCLUDED.spreadsheet_id,
		    tab_name = EXCLUDED.tab_name,
		    active = EXCLUDED.active,
		    updated_at = now()
		RETURNING id, created_at, updated_at
	`, s.StudioID, s.SpreadsheetID, s.TabName, s.Active)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("save sheets settings: %w", err)
	}
	return nil
}

// ----- external leads sheet settings (read-only, third-party sheet) -----

func (r *Repo) GetExternalLeadsSheetSettings(ctx context.Context, studioID uuid.UUID) (*ExternalLeadsSheetSettings, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, spreadsheet_id, tab_name, name_column, first_name_column, last_name_column,
		       email_column, phone_column, source_column, notes_column, date_column, hot_lead_column,
		       trial_purchased_column, continue_ai_after_greeting, active, created_at, updated_at
		FROM studio_external_leads_sheet_settings
		WHERE studio_id = $1
	`, studioID)
	var s ExternalLeadsSheetSettings
	if err := row.Scan(&s.ID, &s.StudioID, &s.SpreadsheetID, &s.TabName, &s.NameColumn, &s.FirstNameColumn,
		&s.LastNameColumn, &s.EmailColumn, &s.PhoneColumn, &s.SourceColumn, &s.NotesColumn, &s.DateColumn,
		&s.HotLeadColumn, &s.TrialPurchasedColumn, &s.ContinueAIAfterGreeting, &s.Active,
		&s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get external leads sheet settings: %w", err)
	}
	return &s, nil
}

func (r *Repo) SaveExternalLeadsSheetSettings(ctx context.Context, s *ExternalLeadsSheetSettings) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO studio_external_leads_sheet_settings
			(studio_id, spreadsheet_id, tab_name, name_column, first_name_column, last_name_column,
			 email_column, phone_column, source_column, notes_column, date_column, hot_lead_column,
			 trial_purchased_column, continue_ai_after_greeting, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (studio_id) DO UPDATE
		SET spreadsheet_id = EXCLUDED.spreadsheet_id,
		    tab_name = EXCLUDED.tab_name,
		    name_column = EXCLUDED.name_column,
		    first_name_column = EXCLUDED.first_name_column,
		    last_name_column = EXCLUDED.last_name_column,
		    email_column = EXCLUDED.email_column,
		    phone_column = EXCLUDED.phone_column,
		    source_column = EXCLUDED.source_column,
		    notes_column = EXCLUDED.notes_column,
		    date_column = EXCLUDED.date_column,
		    hot_lead_column = EXCLUDED.hot_lead_column,
		    trial_purchased_column = EXCLUDED.trial_purchased_column,
		    continue_ai_after_greeting = EXCLUDED.continue_ai_after_greeting,
		    active = EXCLUDED.active,
		    updated_at = now()
		RETURNING id, created_at, updated_at
	`, s.StudioID, s.SpreadsheetID, s.TabName, s.NameColumn, s.FirstNameColumn, s.LastNameColumn,
		s.EmailColumn, s.PhoneColumn, s.SourceColumn, s.NotesColumn, s.DateColumn,
		s.HotLeadColumn, s.TrialPurchasedColumn, s.ContinueAIAfterGreeting, s.Active)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("save external leads sheet settings: %w", err)
	}
	return nil
}

// ListActiveExternalLeadsSheetSettings returns every studio's external-sheet
// config where polling is enabled. Used by the inbound import worker to know
// which spreadsheets to poll.
func (r *Repo) ListActiveExternalLeadsSheetSettings(ctx context.Context) ([]ExternalLeadsSheetSettings, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, spreadsheet_id, tab_name, name_column, first_name_column, last_name_column,
		       email_column, phone_column, source_column, notes_column, date_column, hot_lead_column,
		       trial_purchased_column, continue_ai_after_greeting, active, created_at, updated_at
		FROM studio_external_leads_sheet_settings
		WHERE active = true AND spreadsheet_id != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("list active external leads sheet settings: %w", err)
	}
	defer rows.Close()

	var out []ExternalLeadsSheetSettings
	for rows.Next() {
		var s ExternalLeadsSheetSettings
		if err := rows.Scan(&s.ID, &s.StudioID, &s.SpreadsheetID, &s.TabName, &s.NameColumn, &s.FirstNameColumn,
			&s.LastNameColumn, &s.EmailColumn, &s.PhoneColumn, &s.SourceColumn, &s.NotesColumn, &s.DateColumn,
			&s.HotLeadColumn, &s.TrialPurchasedColumn, &s.ContinueAIAfterGreeting, &s.Active,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan external leads sheet settings: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetExternalSheetImportWatermark returns the last row number already
// imported for a given spreadsheet+tab (1 if never polled before, i.e. start
// at the first data row).
func (r *Repo) GetExternalSheetImportWatermark(ctx context.Context, spreadsheetID, tabName string) (int, error) {
	var lastRow int
	err := r.pool.QueryRow(ctx, `
		SELECT last_row_imported FROM external_sheet_import_log
		WHERE spreadsheet_id = $1 AND tab_name = $2
	`, spreadsheetID, tabName).Scan(&lastRow)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 1, nil
		}
		return 0, fmt.Errorf("get external sheet import watermark: %w", err)
	}
	return lastRow, nil
}

func (r *Repo) SetExternalSheetImportWatermark(ctx context.Context, spreadsheetID, tabName string, studioID uuid.UUID, rowNum int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO external_sheet_import_log (spreadsheet_id, tab_name, last_row_imported, studio_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (spreadsheet_id, tab_name) DO UPDATE
		SET last_row_imported = EXCLUDED.last_row_imported, updated_at = now()
	`, spreadsheetID, tabName, rowNum, studioID)
	if err != nil {
		return fmt.Errorf("set external sheet import watermark: %w", err)
	}
	return nil
}

func (r *Repo) UpdateAutoContactStage(ctx context.Context, studioID, id uuid.UUID, stage string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads SET auto_contact_stage = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, stage)
	if err != nil {
		return fmt.Errorf("update auto_contact_stage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

func (r *Repo) UpdateAutoContactStageTx(ctx context.Context, tx pgx.Tx, studioID, id uuid.UUID, stage string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE leads SET auto_contact_stage = $3, updated_at = now()
		WHERE studio_id = $1 AND id = $2
	`, studioID, id, stage)
	if err != nil {
		return fmt.Errorf("update auto_contact_stage tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrLeadNotFound
	}
	return nil
}

func sanitizeDate(d string) string {
	matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, d)
	if !matched {
		return "1970-01-01"
	}
	return d
}

func (r *Repo) GetAnalytics(ctx context.Context, studioID uuid.UUID, durationDays int, startDate, endDate string) (*AnalyticsSummary, error) {
	// Parse duration interval
	var dateFilter string
	if startDate != "" && endDate != "" {
		dateFilter = fmt.Sprintf("AND created_at >= '%s'::timestamp AND created_at <= '%s 23:59:59'::timestamp", sanitizeDate(startDate), sanitizeDate(endDate))
	} else if durationDays > 0 {
		if durationDays == 1 {
			dateFilter = "AND created_at >= CURRENT_DATE - INTERVAL '1 day'"
		} else {
			dateFilter = fmt.Sprintf("AND created_at >= CURRENT_DATE - INTERVAL '%d days'", durationDays)
		}
	}

	var args []any
	var cond string
	if studioID != uuid.Nil {
		cond = "WHERE studio_id = $1"
		args = append(args, studioID)
	} else {
		cond = "WHERE 1=1"
	}

	// 1. Lead counts by status
	qStatus := fmt.Sprintf(`
		SELECT status, COUNT(*) 
		FROM leads 
		%s %s 
		GROUP BY status`, cond, dateFilter)
	rows, err := r.pool.Query(ctx, qStatus, args...)
	if err != nil {
		return nil, fmt.Errorf("analytics status counts: %w", err)
	}
	defer rows.Close()

	var totalLeads, newLeads, trialLeads, memberLeads, droppedLeads, pausedLeads int
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		totalLeads += count
		switch status {
		case "new":
			newLeads = count
		case "trial_booked":
			trialLeads = count
		case "member":
			memberLeads = count
		case "dropped":
			droppedLeads = count
		case "paused":
			pausedLeads = count
		}
	}

	// 2. Unresponded messages
	var unresponded int
	var qUnresponded string
	if studioID != uuid.Nil {
		qUnresponded = "SELECT COUNT(*) FROM conversations WHERE studio_id = $1 AND status = 'open' AND last_message_direction = 'inbound'"
	} else {
		qUnresponded = "SELECT COUNT(*) FROM conversations WHERE status = 'open' AND last_message_direction = 'inbound'"
	}
	err = r.pool.QueryRow(ctx, qUnresponded, args...).Scan(&unresponded)
	if err != nil {
		return nil, fmt.Errorf("analytics unresponded: %w", err)
	}

	// 3. Followups required
	var qFollowups string
	if studioID != uuid.Nil {
		qFollowups = fmt.Sprintf("SELECT COUNT(*) FROM leads WHERE studio_id = $1 AND status IN ('new', 'contacted') AND contact_attempts < 3 %s", dateFilter)
	} else {
		qFollowups = fmt.Sprintf("SELECT COUNT(*) FROM leads WHERE status IN ('new', 'contacted') AND contact_attempts < 3 %s", dateFilter)
	}
	var followups int
	err = r.pool.QueryRow(ctx, qFollowups, args...).Scan(&followups)
	if err != nil {
		return nil, fmt.Errorf("analytics followups: %w", err)
	}

	// 4. Avg response time lapse
	var avgResponseTime float64
	var studioMsgCond string
	if studioID != uuid.Nil {
		studioMsgCond = "AND m1.studio_id = $1"
	} else {
		studioMsgCond = ""
	}
	qResponseTime := fmt.Sprintf(`
		WITH msg_pairs AS (
			SELECT 
				m1.created_at as inbound_time,
				MIN(m2.created_at) as outbound_time
			FROM messages m1
			JOIN messages m2 ON m1.conversation_id = m2.conversation_id 
				AND m2.direction = 'outbound' 
				AND m2.created_at > m1.created_at
			WHERE m1.direction = 'inbound'
			  %s
			  %s
			GROUP BY m1.id, m1.created_at
		)
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (outbound_time - inbound_time))), 0) FROM msg_pairs`,
		studioMsgCond,
		strings.ReplaceAll(dateFilter, "created_at", "m1.created_at"))
	err = r.pool.QueryRow(ctx, qResponseTime, args...).Scan(&avgResponseTime)
	if err != nil {
		return nil, fmt.Errorf("analytics response time: %w", err)
	}

	// 5. Lead to trial time lapse
	var leadToTrialTime float64
	var qLeadToTrial string
	if studioID != uuid.Nil {
		qLeadToTrial = fmt.Sprintf("SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at))), 0) FROM leads WHERE studio_id = $1 AND status IN ('trial_booked', 'member') %s", dateFilter)
	} else {
		qLeadToTrial = fmt.Sprintf("SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at))), 0) FROM leads WHERE status IN ('trial_booked', 'member') %s", dateFilter)
	}
	err = r.pool.QueryRow(ctx, qLeadToTrial, args...).Scan(&leadToTrialTime)
	if err != nil {
		return nil, fmt.Errorf("analytics lead to trial: %w", err)
	}

	// 6. Trial to member time lapse
	var trialToMemberTime float64
	var qTrialToMember string
	if studioID != uuid.Nil {
		qTrialToMember = fmt.Sprintf("SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - COALESCE(last_contacted_at, created_at)))), 0) FROM leads WHERE studio_id = $1 AND status = 'member' %s", dateFilter)
	} else {
		qTrialToMember = fmt.Sprintf("SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - COALESCE(last_contacted_at, created_at)))), 0) FROM leads WHERE status = 'member' %s", dateFilter)
	}
	err = r.pool.QueryRow(ctx, qTrialToMember, args...).Scan(&trialToMemberTime)
	if err != nil {
		return nil, fmt.Errorf("analytics trial to member: %w", err)
	}

	// 7. Campaign metrics
	var campaignCond string
	if studioID != uuid.Nil {
		campaignCond = "WHERE c.studio_id = $1"
	} else {
		campaignCond = ""
	}
	qCampaigns := fmt.Sprintf(`
		SELECT 
			c.id, c.name, c.slug,
			COUNT(l.id) as total_leads,
			COUNT(CASE WHEN l.status IN ('trial_booked', 'member') THEN 1 END) as converted_leads
		FROM campaigns c
		LEFT JOIN leads l ON l.campaign_id = c.id %s
		%s
		GROUP BY c.id, c.name, c.slug
		ORDER BY total_leads DESC`, strings.ReplaceAll(dateFilter, "created_at", "l.created_at"), campaignCond)
	rowsC, err := r.pool.Query(ctx, qCampaigns, args...)
	if err != nil {
		return nil, fmt.Errorf("analytics campaigns: %w", err)
	}
	defer rowsC.Close()

	byCampaign := []CampaignAnalytics{}
	for rowsC.Next() {
		var ca CampaignAnalytics
		var total, converted int
		if err := rowsC.Scan(&ca.ID, &ca.Name, &ca.Slug, &total, &converted); err != nil {
			return nil, err
		}
		ca.TotalLeads = total
		ca.ConvertedLeads = converted
		if total > 0 {
			ca.ConversionRate = float64(converted) / float64(total) * 100
		}
		byCampaign = append(byCampaign, ca)
	}

	// 8. Platform metrics
	var platformCond string
	if studioID != uuid.Nil {
		platformCond = "WHERE l.studio_id = $1"
	} else {
		platformCond = "WHERE 1=1"
	}
	qPlatforms := fmt.Sprintf(`
		SELECT 
			CASE 
				WHEN lower(l.source) LIKE '%%instagram%%' OR lower(l.referrer) LIKE '%%instagram.com%%' OR lower(l.referrer) LIKE '%%ig.me%%' THEN 'Instagram'
				WHEN lower(l.source) LIKE '%%tiktok%%' OR lower(l.referrer) LIKE '%%tiktok.com%%' THEN 'TikTok'
				WHEN lower(l.source) LIKE '%%youtube%%' OR lower(l.referrer) LIKE '%%youtube.com%%' OR lower(l.referrer) LIKE '%%youtu.be%%' THEN 'YouTube'
				WHEN lower(l.source) LIKE '%%facebook%%' OR lower(l.referrer) LIKE '%%facebook.com%%' OR lower(l.source) LIKE '%%messenger%%' OR lower(l.referrer) LIKE '%%m.me%%' THEN 'Facebook'
				WHEN lower(l.source) LIKE '%%google%%' OR lower(l.referrer) LIKE '%%google.com%%' OR lower(l.source) LIKE '%%seo%%' THEN 'Google / SEO'
				WHEN lower(l.source) LIKE '%%ad%%' OR lower(l.referrer) LIKE '%%ad%%' OR lower(l.referrer) LIKE '%%gclid%%' OR l.source = 'public_form' THEN 'Paid Ads'
				ELSE 'Direct / Organic'
			END as platform,
			COUNT(l.id) as total_leads,
			COUNT(CASE WHEN l.status IN ('trial_booked', 'member') THEN 1 END) as converted_leads
		FROM leads l
		%s %s
		GROUP BY platform
		ORDER BY total_leads DESC`, platformCond, dateFilter)
	rowsP, err := r.pool.Query(ctx, qPlatforms, args...)
	if err != nil {
		return nil, fmt.Errorf("analytics platforms: %w", err)
	}
	defer rowsP.Close()

	byPlatform := []PlatformAnalytics{}
	for rowsP.Next() {
		var pa PlatformAnalytics
		var total, converted int
		if err := rowsP.Scan(&pa.Platform, &total, &converted); err != nil {
			return nil, err
		}
		pa.TotalLeads = total
		pa.ConvertedLeads = converted
		if total > 0 {
			pa.ConversionRate = float64(converted) / float64(total) * 100
		}
		byPlatform = append(byPlatform, pa)
	}

	trialToMemberRate := 0.0
	if (trialLeads + memberLeads) > 0 {
		trialToMemberRate = float64(memberLeads) / float64(trialLeads+memberLeads) * 100
	}

	droppedRate := 0.0
	if totalLeads > 0 {
		droppedRate = float64(droppedLeads) / float64(totalLeads) * 100
	}

	pausedRate := 0.0
	if totalLeads > 0 {
		pausedRate = float64(pausedLeads) / float64(totalLeads) * 100
	}

	return &AnalyticsSummary{
		TotalLeads:                 totalLeads,
		NewLeads:                   newLeads,
		TrialBookedLeads:           trialLeads,
		MemberLeads:                memberLeads,
		DroppedLeads:               droppedLeads,
		PausedLeads:                pausedLeads,
		TrialToMemberRate:          trialToMemberRate,
		DroppedRate:                droppedRate,
		PausedRate:                 pausedRate,
		FollowupsRequired:          followups,
		UnrespondedMessages:        unresponded,
		AvgResponseTimeLapseSecs:   avgResponseTime,
		LeadToTrialTimeLapseSecs:   leadToTrialTime,
		TrialToMemberTimeLapseSecs: trialToMemberTime,
		ByCampaign:                 byCampaign,
		ByPlatform:                 byPlatform,
	}, nil
}

// FindLeadByEmail looks up the most-recent lead for a given email within a studio.
// Returns ErrLeadNotFound when no match exists.
func (r *Repo) FindLeadByEmail(ctx context.Context, studioID uuid.UUID, email string) (*Lead, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT l.id, l.studio_id, s.name, s.slug, l.campaign_id, c.name, c.slug,
		       l.name, COALESCE(l.first_name, ''), COALESCE(l.last_name, ''), l.email, l.phone, l.fitness_plan, l.goals,
		       l.source, l.status, l.currency, l.notes, l.contact_attempts, l.last_contacted_at, l.contact_made, l.hot_lead, l.trial_purchased, l.auto_contact_stage,
		       COALESCE(l.assigned_to, ''), l.trial_attended, l.member_sold, l.monthly_fee, COALESCE(l.offer, ''), COALESCE(l.further_notes, ''),
		       l.dnd_enabled, l.created_at, l.updated_at
		FROM leads l
		JOIN campaigns c ON c.id = l.campaign_id
		JOIN studios s ON s.id = l.studio_id
		WHERE l.studio_id = $1 AND LOWER(l.email) = LOWER($2)
		ORDER BY l.created_at DESC
		LIMIT 1
	`, studioID, email)
	var l Lead
	if err := row.Scan(&l.ID, &l.StudioID, &l.StudioName, &l.StudioSlug, &l.CampaignID, &l.CampaignName, &l.CampaignSlug,
		&l.Name, &l.FirstName, &l.LastName, &l.Email, &l.Phone, &l.FitnessPlan, &l.Goals,
		&l.Source, &l.Status, &l.Currency, &l.Notes, &l.ContactAttempts, &l.LastContactedAt, &l.ContactMade, &l.HotLead, &l.TrialPurchased, &l.AutoContactStage,
		&l.AssignedTo, &l.TrialAttended, &l.MemberSold, &l.MonthlyFee, &l.Offer, &l.FurtherNotes, &l.DNDEnabled, &l.CreatedAt, &l.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLeadNotFound
		}
		return nil, fmt.Errorf("find lead by email: %w", err)
	}
	return &l, nil
}

// MarkGlofoxFirstSessionNotified atomically sets glofox_first_session_notified_at on a lead
// only if it has not already been set. Returns true if this was the first time (i.e., the
// row was actually updated), false if already marked (duplicate guard).
func (r *Repo) MarkGlofoxFirstSessionNotified(ctx context.Context, leadID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE leads
		SET glofox_first_session_notified_at = now(), updated_at = now()
		WHERE id = $1 AND glofox_first_session_notified_at IS NULL
	`, leadID)
	if err != nil {
		return false, fmt.Errorf("mark glofox first session notified: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repo) GetUniqueSources(ctx context.Context, studioID uuid.UUID) ([]string, error) {
	var rows pgx.Rows
	var err error
	if studioID == uuid.Nil {
		rows, err = r.pool.Query(ctx, `
			SELECT DISTINCT COALESCE(source, '') 
			FROM leads 
			WHERE source IS NOT NULL AND source <> ''
			ORDER BY 1
		`)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT DISTINCT COALESCE(source, '') 
			FROM leads 
			WHERE studio_id = $1 AND source IS NOT NULL AND source <> ''
			ORDER BY 1
		`, studioID)
	}
	if err != nil {
		return nil, fmt.Errorf("unique sources query: %w", err)
	}
	defer rows.Close()

	var sources []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}
