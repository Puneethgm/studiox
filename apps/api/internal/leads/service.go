package leads

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

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

func (s *Service) ListCampaigns(ctx context.Context, studioID uuid.UUID) ([]Campaign, error) {
	return s.repo.ListCampaigns(ctx, studioID)
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

// ----- leads -----

type SubmitLeadInput struct {
	StudioSlug   string
	CampaignSlug string
	Name         string
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
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Phone = strings.TrimSpace(in.Phone)
	in.FitnessPlan = strings.TrimSpace(in.FitnessPlan)
	in.Goals = strings.TrimSpace(in.Goals)

	errs := map[string]string{}
	if in.Name == "" {
		errs["name"] = "required"
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
		StudioID:    c.StudioID,
		StudioName:  c.StudioName,
		StudioSlug:  c.StudioSlug,
		CampaignID:  c.ID,
		Name:        in.Name,
		Email:       in.Email,
		Phone:       in.Phone,
		FitnessPlan: in.FitnessPlan,
		Goals:       in.Goals,
		Source:      "public_form",
		Referrer:    in.Referrer,
		UserAgent:   in.UserAgent,
		IPAddress:   ip,
	}
	if err := s.repo.CreateLeadWithOutbox(ctx, l, sheetsDestination); err != nil {
		return nil, nil, err
	}
	l.CampaignName = c.Name
	l.CampaignSlug = c.Slug
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

func (s *Service) UpdateLead(ctx context.Context, studioID, id uuid.UUID, status LeadStatus, notes string) error {
	if !status.Valid() {
		return fmt.Errorf("invalid status %q", status)
	}
	return s.repo.UpdateLead(ctx, studioID, id, status, notes)
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
