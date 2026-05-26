package leads

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

const sheetsDestination = "google_sheets"

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

// ----- campaigns -----

type CreateCampaignInput struct {
	Slug         string
	Name         string
	Description  string
	FitnessPlans []string
}

func (s *Service) CreateCampaign(ctx context.Context, studioID, userID uuid.UUID, in CreateCampaignInput) (*Campaign, map[string]string, error) {
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	plans := normalizePlans(in.FitnessPlans)

	errs := map[string]string{}
	if in.Name == "" {
		errs["name"] = "required"
	}
	if len(plans) == 0 {
		errs["fitnessPlans"] = "at least one plan is required"
	}
	if in.Slug == "" {
		in.Slug = generateSlug(in.Name)
	} else if !slugRe.MatchString(in.Slug) {
		errs["slug"] = "lowercase letters, digits, and hyphens only"
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}

	c := &Campaign{
		StudioID:     studioID,
		Slug:         in.Slug,
		Name:         in.Name,
		Description:  in.Description,
		FitnessPlans: plans,
		Active:       true,
		CreatedBy:    userID,
	}
	if err := s.repo.CreateCampaign(ctx, c); err != nil {
		return nil, nil, err
	}
	return c, nil, nil
}

func (s *Service) ListCampaigns(ctx context.Context, studioID uuid.UUID, limit, offset int) ([]Campaign, int, error) {
	return s.repo.ListCampaigns(ctx, studioID, limit, offset)
}

func (s *Service) GetCampaign(ctx context.Context, studioID, id uuid.UUID) (*Campaign, error) {
	return s.repo.GetCampaign(ctx, studioID, id)
}

func (s *Service) GetPublicCampaign(ctx context.Context, studioSlug, campaignSlug string) (*Campaign, error) {
	return s.repo.GetActiveCampaignByStudioAndSlug(ctx, studioSlug, campaignSlug)
}

func (s *Service) SetCampaignActive(ctx context.Context, studioID, id uuid.UUID, active bool) error {
	return s.repo.SetCampaignActive(ctx, studioID, id, active)
}

func (s *Service) UpdateCampaignFitnessPlans(ctx context.Context, studioID, id uuid.UUID, fitnessPlans []string) (*Campaign, map[string]string, error) {
	plans := normalizePlans(fitnessPlans)
	if len(plans) == 0 {
		return nil, map[string]string{"fitnessPlans": "at least one plan is required"}, nil
	}
	if err := s.repo.UpdateCampaignFitnessPlans(ctx, studioID, id, plans); err != nil {
		return nil, nil, err
	}
	updated, err := s.repo.GetCampaign(ctx, studioID, id)
	if err != nil {
		return nil, nil, err
	}
	return updated, nil, nil
}

// ----- leads -----

type SubmitLeadInput struct {
	StudioSlug   string
	CampaignSlug string
	Name         string
	FirstName    string
	LastName     string
	Email        string
	Phone        string
	FitnessPlan  string
	Goals        string
	Referrer     string
	UserAgent    string
	IPAddress    string
}

func (s *Service) SubmitPublicLead(ctx context.Context, in SubmitLeadInput) (*Lead, map[string]string, error) {
	c, err := s.repo.GetActiveCampaignByStudioAndSlug(ctx, in.StudioSlug, in.CampaignSlug)
	if err != nil {
		return nil, nil, err
	}

	in.Name = strings.TrimSpace(in.Name)
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.FitnessPlan = strings.TrimSpace(in.FitnessPlan)
	in.Goals = strings.TrimSpace(in.Goals)

	// Combine or split names depending on what's provided
	if in.Name == "" && (in.FirstName != "" || in.LastName != "") {
		in.Name = strings.TrimSpace(in.FirstName + " " + in.LastName)
	} else if in.Name != "" && in.FirstName == "" && in.LastName == "" {
		parts := strings.SplitN(in.Name, " ", 2)
		in.FirstName = parts[0]
		if len(parts) > 1 {
			in.LastName = parts[1]
		}
	}

	errs := map[string]string{}
	if in.FirstName == "" {
		errs["firstName"] = "required"
	}
	if in.LastName == "" {
		errs["lastName"] = "required"
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		errs["email"] = "invalid email"
	}
	if !phoneRe.MatchString(in.Phone) {
		errs["phone"] = "invalid phone number"
	}
	if !planAllowed(c.FitnessPlans, in.FitnessPlan) {
		errs["fitnessPlan"] = "select one of the offered plans"
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}

	var ip *net.IP
	if parsed := net.ParseIP(in.IPAddress); parsed != nil {
		ip = &parsed
	}

	l := &Lead{
		StudioID:     c.StudioID,
		StudioName:   c.StudioName,
		StudioSlug:   c.StudioSlug,
		CampaignID:   c.ID,
		CampaignName: c.Name,
		CampaignSlug: c.Slug,
		Name:         in.Name,
		FirstName:    in.FirstName,
		LastName:     in.LastName,
		Email:        in.Email,
		Phone:        in.Phone,
		FitnessPlan:  in.FitnessPlan,
		Goals:        in.Goals,
		Source:       "public_form",
		Referrer:     in.Referrer,
		UserAgent:    in.UserAgent,
		IPAddress:    ip,
	}
	// If the selected plan indicates a trial booking, set status accordingly.
	// Accept a wider variety of labels (e.g. "Book a trial", "trial booking")
	normalizedPlan := strings.ToLower(strings.TrimSpace(in.FitnessPlan))
	if strings.Contains(normalizedPlan, "trial") || strings.Contains(normalizedPlan, "trail") {
		l.Status = StatusTrialBooked
	} else {
		l.Status = StatusNew
	}
	if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination); err != nil {
		return nil, nil, err
	}
	return l, nil, nil
}

func (s *Service) ListLeads(ctx context.Context, studioID uuid.UUID, f ListLeadsFilter) ([]Lead, int, error) {
	return s.repo.ListLeads(ctx, studioID, f)
}

func (s *Service) Stats(ctx context.Context, studioID uuid.UUID) (*LeadStats, error) {
	return s.repo.Stats(ctx, studioID)
}

func (s *Service) GetLead(ctx context.Context, studioID, id uuid.UUID) (*Lead, error) {
	return s.repo.GetLead(ctx, studioID, id)
}

func (s *Service) UpdateLead(ctx context.Context, studioID, id uuid.UUID, status LeadStatus, currency string, notes string, contactMade, hotLead, trialPurchased bool, firstName, lastName string, assignedTo string, trialAttended, memberSold bool, monthlyFee float64, offer, furtherNotes string) error {
	if !status.Valid() {
		return fmt.Errorf("invalid status %q", status)
	}
	if status == StatusTrialBooked {
		trialPurchased = true
	} else if status == StatusMember {
		memberSold = true
	}
	return s.repo.UpdateLead(ctx, studioID, id, status, currency, notes, contactMade, hotLead, trialPurchased, firstName, lastName, assignedTo, trialAttended, memberSold, monthlyFee, offer, furtherNotes)
}

func (s *Service) GetUniqueSources(ctx context.Context, studioID uuid.UUID) ([]string, error) {
	return s.repo.GetUniqueSources(ctx, studioID)
}

func (s *Service) GetSheetsSettings(ctx context.Context, studioID uuid.UUID) (*StudioSheetsSettings, error) {
	return s.repo.GetSheetsSettings(ctx, studioID)
}

func (s *Service) SaveSheetsSettings(ctx context.Context, studioID uuid.UUID, spreadsheetID, tabName string, active bool) (*StudioSheetsSettings, error) {
	settings := &StudioSheetsSettings{
		StudioID:      studioID,
		SpreadsheetID: strings.TrimSpace(spreadsheetID),
		TabName:       strings.TrimSpace(tabName),
		Active:        active,
	}
	if settings.TabName == "" {
		settings.TabName = "Leads"
	}
	if err := s.repo.SaveSheetsSettings(ctx, settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// ----- helpers -----

var (
	slugRe  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	phoneRe = regexp.MustCompile(`^\+?[0-9\s\-()]{7,20}$`)
)

func normalizePlans(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

func planAllowed(plans []string, picked string) bool {
	want := strings.ToLower(strings.TrimSpace(picked))
	for _, p := range plans {
		if strings.ToLower(strings.TrimSpace(p)) == want {
			return true
		}
	}
	return false
}

func generateSlug(name string) string {
	base := strings.ToLower(name)
	base = nonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "campaign"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	return base + "-" + randomSuffix(4)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func randomSuffix(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return strings.ToLower(enc)[:n]
}

func (s *Service) ImportLeads(ctx context.Context, studioID uuid.UUID, defaultCampaignID uuid.UUID, rows [][]string) (int, error) {
	if len(rows) < 2 {
		return 0, fmt.Errorf("no data rows found")
	}

	headerRow := rows[0]
	mapping := mapHeaders(headerRow)

	// Fetch default campaign details to populate the leads
	defaultCamp, err := s.repo.GetCampaign(ctx, studioID, defaultCampaignID)
	if err != nil {
		return 0, fmt.Errorf("default campaign not found: %w", err)
	}

	importedCount := 0
	for rIdx := 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if len(row) == 0 {
			continue
		}

		// Helper to safely get column by mapped key
		getVal := func(key string, colIdx int) string {
			if idx, ok := mapping[key]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			if len(mapping) > 0 {
				return ""
			}
			if colIdx >= 0 && colIdx < len(row) {
				return strings.TrimSpace(row[colIdx])
			}
			return ""
		}

		firstName := getVal("firstName", 0)
		lastName := getVal("lastName", 1)
		email := getVal("email", 2)
		phone := getVal("phone", 3)
		plan := getVal("plan", 4)
		goals := getVal("goals", 5)
		notes := getVal("notes", 6)
		statusStr := getVal("status", 7)
		name := getVal("name", -1)

		if email == "" && phone == "" {
			// Skip rows without any contact info
			continue
		}

		// If name is empty but first/last name are provided
		if name == "" {
			name = strings.TrimSpace(firstName + " " + lastName)
		} else if firstName == "" && lastName == "" {
			// Split full name
			parts := strings.SplitN(name, " ", 2)
			firstName = parts[0]
			if len(parts) > 1 {
				lastName = parts[1]
			}
		}

		// Validate email if present
		email = strings.ToLower(email)
		if email != "" {
			if _, err := mail.ParseAddress(email); err != nil {
				email = "" // clear invalid email so it doesn't block insertion
			}
		}

		// Basic phone cleaning (keep digits, spaces, plus, hyphens)
		if phone != "" {
			phone = phoneRe.FindString(phone)
		}

		// Determine status
		status := StatusNew
		if statusStr != "" {
			sTemp := LeadStatus(strings.ToLower(statusStr))
			if sTemp.Valid() {
				status = sTemp
			}
		} else if strings.Contains(strings.ToLower(plan), "trial") {
			status = StatusTrialBooked
		}

		// Map to a Lead structure
		l := &Lead{
			StudioID:     studioID,
			StudioName:   defaultCamp.StudioName,
			StudioSlug:   defaultCamp.StudioSlug,
			CampaignID:   defaultCamp.ID,
			CampaignName: defaultCamp.Name,
			CampaignSlug: defaultCamp.Slug,
			Name:         name,
			FirstName:    firstName,
			LastName:     lastName,
			Email:        email,
			Phone:        phone,
			FitnessPlan:  plan,
			Goals:        goals,
			Notes:        notes,
			Status:       status,
			Source:       "import",
		}

		if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination); err != nil {
			return importedCount, fmt.Errorf("row %d import: %w", rIdx, err)
		}
		importedCount++
	}

	return importedCount, nil
}

func mapHeaders(headerRow []string) map[string]int {
	mapping := make(map[string]int)
	for i, h := range headerRow {
		h = strings.ToLower(strings.TrimSpace(h))
		if strings.Contains(h, "first") && strings.Contains(h, "name") {
			mapping["firstName"] = i
		} else if strings.Contains(h, "last") && strings.Contains(h, "name") {
			mapping["lastName"] = i
		} else if h == "name" || strings.Contains(h, "full name") {
			mapping["name"] = i
		} else if strings.Contains(h, "email") {
			mapping["email"] = i
		} else if strings.Contains(h, "phone") || strings.Contains(h, "number") || strings.Contains(h, "contact") {
			mapping["phone"] = i
		} else if strings.Contains(h, "plan") {
			mapping["plan"] = i
		} else if strings.Contains(h, "goal") {
			mapping["goals"] = i
		} else if strings.Contains(h, "note") {
			mapping["notes"] = i
		} else if strings.Contains(h, "status") {
			mapping["status"] = i
		} else if strings.Contains(h, "campaign") {
			mapping["campaign"] = i
		}
	}
	return mapping
}

func (s *Service) BookTrialSlot(ctx context.Context, leadID uuid.UUID, slot string) error {
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var studioID uuid.UUID
	var name, notes, status, phone string
	err = tx.QueryRow(ctx, `
		SELECT studio_id, name, notes, status, phone FROM leads
		WHERE id = $1
	`, leadID).Scan(&studioID, &name, &notes, &status, &phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if status == "trial_booked" || strings.Contains(notes, "[Selected Trial Slot]:") {
		return nil
	}

	newNotes := strings.TrimSpace(notes + "\n[Selected Trial Slot]: " + slot)

	_, err = tx.Exec(ctx, `
		UPDATE leads
		SET status = 'trial_booked', notes = $2, trial_purchased = true, auto_contact_stage = 'completed', updated_at = now()
		WHERE id = $1
	`, leadID, newNotes)
	if err != nil {
		return err
	}

	// Try to find a conversation for this lead to send the WhatsApp confirmation
	var convID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM conversations
		WHERE studio_id = $1 AND lead_id = $2
		LIMIT 1
	`, studioID, leadID).Scan(&convID)
	if err == nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO outbound_jobs (studio_id, conversation_id, body, attachments,
			                           source_kind, source_ref, scheduled_for, next_attempt_at)
			VALUES ($1, $2, 'Tnak you our team will reach u out ', '[]'::jsonb, 'automation', $3, now(), now())
		`, studioID, convID, fmt.Sprintf("lead:%s:auto_reply:completed", leadID.String()))
		if err != nil {
			return err
		}
	}

	// Enqueue Google Sheets update
	l, err := s.repo.GetLeadTx(ctx, tx, studioID, leadID)
	if err == nil {
		payload, err := json.Marshal(l)
		if err == nil {
			_, _ = tx.Exec(ctx, `
				INSERT INTO outbox (aggregate_type, aggregate_id, event_type, destination, payload)
				VALUES ('lead', $1, 'lead.updated', 'google_sheets', $2)
			`, l.ID, payload)
		}
	}

	return tx.Commit(ctx)
}

func (s *Service) GetAnalytics(ctx context.Context, studioID uuid.UUID, durationDays int, startDate, endDate string) (*AnalyticsSummary, error) {
	return s.repo.GetAnalytics(ctx, studioID, durationDays, startDate, endDate)
}

