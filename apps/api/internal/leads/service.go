package leads

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/integrations/glofox"
)

var ErrNotFound = errors.New("not found")

// sheetsIDRe extracts the spreadsheet ID from a full Google Sheets URL.
// e.g. https://docs.google.com/spreadsheets/d/{ID}/edit → {ID}
var sheetsIDRe = regexp.MustCompile(`/spreadsheets/d/([a-zA-Z0-9_-]+)`)

func extractSpreadsheetID(raw string) string {
	raw = strings.TrimSpace(raw)
	if m := sheetsIDRe.FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	return raw
}

const sheetsDestination = "google_sheets"

// CancelPendingMessagesFunc cancels every still-pending scheduled/automated
// message for a lead (across all of its conversations) — called when DND is
// turned on so already-queued follow-ups don't slip through. Wired in from
// the messaging package via a callback to keep the import direction one-way.
type CancelPendingMessagesFunc func(ctx context.Context, studioID, leadID uuid.UUID) (int, error)

type Service struct {
	repo                  *Repo
	glofox                *glofox.Client
	cancelPendingMessages CancelPendingMessagesFunc
}

func NewService(repo *Repo, gf *glofox.Client) *Service {
	return &Service{repo: repo, glofox: gf}
}

// SetCancelPendingMessagesFunc wires in the messaging package's job-cancellation
// callback after construction, mirroring how studios wires brandLookup into
// identity in main.go.
func (s *Service) SetCancelPendingMessagesFunc(fn CancelPendingMessagesFunc) {
	s.cancelPendingMessages = fn
}

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
	if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination, false); err != nil {
		return nil, nil, err
	}
	return l, nil, nil
}

// TrialSignupInput is the payload for the studio-wide (no-lead-attached)
// trial signup link — a static URL a studio can share manually (WhatsApp
// broadcast, bio link, etc.) that creates a brand-new lead on submit, unlike
// the per-lead trial-details link which requires an existing lead ID.
type TrialSignupInput struct {
	StudioSlug  string
	FullName    string
	Phone       string
	Gender      string
	DateOfBirth string // "YYYY-MM-DD", optional
	Referrer    string
	UserAgent   string
	IPAddress   string
}

// SubmitTrialSignup creates a brand-new lead for a studio from its static
// trial signup link, resolving a default campaign the same way CSV imports
// do (oldest active campaign), since campaign_id is a required FK and this
// link has no campaign context of its own.
func (s *Service) SubmitTrialSignup(ctx context.Context, in TrialSignupInput) (*Lead, map[string]string, error) {
	c, err := s.repo.GetOldestActiveCampaignByStudioSlug(ctx, in.StudioSlug)
	if err != nil {
		return nil, nil, err
	}

	fullName := strings.TrimSpace(in.FullName)
	phone := strings.TrimSpace(in.Phone)

	errs := map[string]string{}
	if fullName == "" {
		errs["fullName"] = "required"
	}
	if !phoneRe.MatchString(phone) {
		errs["phone"] = "invalid phone number"
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}

	firstName, lastName := fullName, ""
	if parts := strings.SplitN(fullName, " ", 2); len(parts) > 1 {
		firstName, lastName = parts[0], parts[1]
	}

	fitnessPlan := "General"
	if len(c.FitnessPlans) > 0 {
		fitnessPlan = c.FitnessPlans[0]
	}

	var dob *time.Time
	if in.DateOfBirth != "" {
		if parsed, err := time.Parse("2006-01-02", in.DateOfBirth); err == nil {
			dob = &parsed
		}
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
		Name:         fullName,
		FirstName:    firstName,
		LastName:     lastName,
		Phone:        phone,
		FitnessPlan:  fitnessPlan,
		Source:       "trial_link",
		Status:       StatusTrialBooked,
		Gender:       strings.TrimSpace(in.Gender),
		DateOfBirth:  dob,
		Referrer:     in.Referrer,
		UserAgent:    in.UserAgent,
		IPAddress:    ip,
	}
	if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination, false); err != nil {
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

// SetDND toggles Do Not Disturb for a lead. Turning it on also cancels every
// already-queued automated message for that lead so nothing slips through
// after the toggle — turning it off never re-schedules anything; automation
// resumes naturally the next time the lead is contacted or replies.
func (s *Service) SetDND(ctx context.Context, studioID, id uuid.UUID, enabled bool) (*Lead, error) {
	if err := s.repo.SetDNDEnabled(ctx, studioID, id, enabled); err != nil {
		return nil, err
	}
	if enabled && s.cancelPendingMessages != nil {
		if _, err := s.cancelPendingMessages(ctx, studioID, id); err != nil {
			return nil, fmt.Errorf("cancel pending messages: %w", err)
		}
	}
	return s.repo.GetLead(ctx, studioID, id)
}

func (s *Service) UpdateLead(ctx context.Context, studioID, id uuid.UUID, status LeadStatus, currency string, notes string, contactMade, hotLead, trialPurchased bool, firstName, lastName, fitnessPlan, assignedTo string, trialAttended, memberSold bool, monthlyFee float64, offer, furtherNotes string) error {
	if !status.Valid() {
		return fmt.Errorf("invalid status %q", status)
	}
	if status == StatusTrialBooked {
		trialPurchased = true
	} else if status == StatusMember {
		memberSold = true
	}

	// Fetch the current lead before updating so we have email + phone for Glofox.
	var existing *Lead
	if s.glofox != nil && (status == StatusTrialBooked || status == StatusMember) {
		if l, err := s.repo.GetLead(ctx, studioID, id); err == nil {
			existing = l
		}
	}

	if err := s.repo.UpdateLead(ctx, studioID, id, status, currency, notes, contactMade, hotLead, trialPurchased, firstName, lastName, fitnessPlan, assignedTo, trialAttended, memberSold, monthlyFee, offer, furtherNotes); err != nil {
		return err
	}

	// Push to Glofox when a lead converts to trial or member.
	// Fire-and-forget: log on error, never block the HTTP response.
	if existing != nil {
		fn := firstName
		if fn == "" {
			fn = existing.FirstName
		}
		ln := lastName
		if ln == "" {
			ln = existing.LastName
		}
		if ln == "" {
			// Glofox rejects the request outright without a last name —
			// many WhatsApp leads only ever give a first name.
			ln = "-"
		}
		gfStatus := glofox.GlofoxStatusTrial
		if status == StatusMember {
			gfStatus = glofox.GlofoxStatusMember
		}
		leadID := id
		leadName := fn + " " + ln
		leadEmail := existing.Email
		leadPhone := existing.Phone
		leadGender := existing.Gender
		leadBirthDate := ""
		if existing.DateOfBirth != nil {
			leadBirthDate = existing.DateOfBirth.Format("2006-01-02")
		}
		go func() {
			gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := s.glofox.CreateLead(gCtx, glofox.CreateLeadInput{
				Email:         leadEmail,
				FirstName:     fn,
				LastName:      ln,
				Phone:         leadPhone,
				LeadStatus:    gfStatus,
				ContactSource: existing.Source,
				Gender:        leadGender,
				BirthDate:     leadBirthDate,
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

	return nil
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
		SpreadsheetID: extractSpreadsheetID(spreadsheetID),
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

func (s *Service) GetExternalLeadsSheetSettings(ctx context.Context, studioID uuid.UUID) (*ExternalLeadsSheetSettings, error) {
	return s.repo.GetExternalLeadsSheetSettings(ctx, studioID)
}

func (s *Service) SaveExternalLeadsSheetSettings(ctx context.Context, studioID uuid.UUID, in ExternalLeadsSheetSettings) (*ExternalLeadsSheetSettings, error) {
	settings := &ExternalLeadsSheetSettings{
		StudioID:        studioID,
		SpreadsheetID:   extractSpreadsheetID(in.SpreadsheetID),
		TabName:         strings.TrimSpace(in.TabName),
		NameColumn:      strings.ToUpper(strings.TrimSpace(in.NameColumn)),
		FirstNameColumn: strings.ToUpper(strings.TrimSpace(in.FirstNameColumn)),
		LastNameColumn:  strings.ToUpper(strings.TrimSpace(in.LastNameColumn)),
		EmailColumn:     strings.ToUpper(strings.TrimSpace(in.EmailColumn)),
		PhoneColumn:     strings.ToUpper(strings.TrimSpace(in.PhoneColumn)),
		SourceColumn:    strings.ToUpper(strings.TrimSpace(in.SourceColumn)),
		NotesColumn:     strings.ToUpper(strings.TrimSpace(in.NotesColumn)),
		DateColumn:      strings.ToUpper(strings.TrimSpace(in.DateColumn)),
		Active:          in.Active,
	}
	if settings.TabName == "" {
		settings.TabName = "Sheet1"
	}
	if settings.NameColumn == "" {
		if settings.FirstNameColumn == "" {
			settings.FirstNameColumn = "A"
		}
		if settings.LastNameColumn == "" {
			settings.LastNameColumn = "B"
		}
	}
	if settings.EmailColumn == "" {
		settings.EmailColumn = "C"
	}
	if settings.PhoneColumn == "" {
		settings.PhoneColumn = "D"
	}
	if err := s.repo.SaveExternalLeadsSheetSettings(ctx, settings); err != nil {
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

		if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination, false); err != nil {
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
			`, l.ID, string(payload))
		}
	}

	return tx.Commit(ctx)
}

func (s *Service) GetAnalytics(ctx context.Context, studioID uuid.UUID, durationDays int, startDate, endDate string) (*AnalyticsSummary, error) {
	return s.repo.GetAnalytics(ctx, studioID, durationDays, startDate, endDate)
}
