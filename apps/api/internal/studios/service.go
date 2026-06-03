package studios

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/identity"
)

type Service struct {
	repo     *Repo
	identity *identity.Repo
}

func NewService(repo *Repo, id *identity.Repo) *Service {
	return &Service{repo: repo, identity: id}
}

// ----- create studio + first admin (atomic) -----

type CreateStudioInput struct {
	Slug          string
	Name          string
	BrandColor    string
	LogoURL       string
	ContactEmail  string
	AdminEmail           string
	AdminPassword        string
	SocialPlannerEnabled bool
}

type CreateStudioResult struct {
	Studio  *Studio   `json:"studio"`
	AdminID uuid.UUID `json:"adminId"`
}

// CreateStudioWithAdmin creates the studio and its first studio_admin in a
// single transaction so a half-provisioned studio (with no admin) can never
// exist.
func (s *Service) CreateStudioWithAdmin(ctx context.Context, in CreateStudioInput) (*CreateStudioResult, map[string]string, error) {
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	in.BrandColor = normalizeHex(in.BrandColor)
	in.LogoURL = strings.TrimSpace(in.LogoURL)
	in.ContactEmail = strings.ToLower(strings.TrimSpace(in.ContactEmail))
	in.AdminEmail = strings.ToLower(strings.TrimSpace(in.AdminEmail))

	errs := map[string]string{}
	if in.Name == "" {
		errs["name"] = "required"
	}
	if in.Slug == "" {
		in.Slug = generateSlug(in.Name)
	} else if !slugRe.MatchString(in.Slug) {
		errs["slug"] = "lowercase letters, digits, and hyphens only"
	}
	if !hexRe.MatchString(in.BrandColor) {
		errs["brandColor"] = "must be a hex color like #7c3aed"
	}
	if _, err := mail.ParseAddress(in.AdminEmail); err != nil {
		errs["adminEmail"] = "invalid email"
	}
	if len(in.AdminPassword) < 8 {
		errs["adminPassword"] = "must be at least 8 characters"
	}
	if in.ContactEmail != "" {
		if _, err := mail.ParseAddress(in.ContactEmail); err != nil {
			errs["contactEmail"] = "invalid email"
		}
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}

	pool := s.repo.Pool()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	studio := &Studio{
		Slug:                 in.Slug,
		Name:                 in.Name,
		BrandColor:           in.BrandColor,
		LogoURL:              in.LogoURL,
		ContactEmail:         in.ContactEmail,
		Active:               true,
		SocialPlannerEnabled: in.SocialPlannerEnabled,
		KnowledgeBaseFiles:   []KnowledgeBaseFile{},
	}
	if err := s.repo.Create(ctx, tx, studio); err != nil {
		if errors.Is(err, ErrSlugTaken) {
			return nil, map[string]string{"slug": "this slug is already in use"}, nil
		}
		return nil, nil, err
	}

	hash, err := identity.HashPassword(in.AdminPassword)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}
	// CreateStudioAdmin uses the pool directly — but inside the same tx we must
	// run it on the tx connection. Inline the insert here to honor atomicity.
	var adminID uuid.UUID
	row := tx.QueryRow(ctx, `
		INSERT INTO users (studio_id, email, password_hash, role)
		VALUES ($1, $2, $3, 'studio_admin')
		RETURNING id
	`, studio.ID, in.AdminEmail, hash)
	if err := row.Scan(&adminID); err != nil {
		// Unique-violation on email
		if isPgUnique(err) {
			return nil, map[string]string{"adminEmail": "this email is already registered"}, nil
		}
		return nil, nil, fmt.Errorf("create studio admin: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO plans (studio_id, plan_name, price_sgd, billing_cycle, features) VALUES 
		($1, 'Trial', 0, 'one_time', ARRAY['Free', 'Limited features', 'Valid for 7 days']),
		($1, 'Basic', 2900, 'monthly', ARRAY['Feature A', 'Feature B', 'Feature C']),
		($1, 'Pro', 9900, 'monthly', ARRAY['All Basic Features', 'Feature D', 'Feature E']),
		($1, 'Pro Plus', 19900, 'monthly', ARRAY['All Pro Features', 'Feature F', 'Feature G'])
	`, studio.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("create default plans: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	return &CreateStudioResult{Studio: studio, AdminID: adminID}, nil, nil
}

// ----- list / get / update -----

func (s *Service) List(ctx context.Context) ([]Studio, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Studio, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Studio, error) {
	return s.repo.GetBySlug(ctx, slug)
}

type UpdateStudioInput struct {
	Name                 string
	BrandColor           string
	LogoURL              string
	ContactEmail         string
	Active               bool
	AvailabilitySlots    []AvailabilitySlot `json:"availabilitySlots"`
	AvailabilityTimezone string             `json:"availabilityTimezone"`
	GeminiAPIKey         string             `json:"geminiApiKey"`
	MetaAppID            string             `json:"metaAppId"`
	MetaAppSecret        string             `json:"metaAppSecret"`
	GoogleClientID       string             `json:"googleClientId"`
	GoogleClientSecret   string             `json:"googleClientSecret"`
	GoogleDeveloperToken string             `json:"googleDeveloperToken"`
	SocialPlannerEnabled bool               `json:"socialPlannerEnabled"`
	KnowledgeBase        string             `json:"knowledgeBase"`
	KnowledgeBaseFiles   []KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	TrialAmountSGD       int                 `json:"trialAmountSgd"`
	
	
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateStudioInput) (map[string]string, error) {
	oldStudio, err := s.repo.GetByID(ctx, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	in.Name = strings.TrimSpace(in.Name)
	in.BrandColor = normalizeHex(in.BrandColor)
	in.LogoURL = strings.TrimSpace(in.LogoURL)
	in.ContactEmail = strings.ToLower(strings.TrimSpace(in.ContactEmail))
	in.GeminiAPIKey = strings.TrimSpace(in.GeminiAPIKey)
	in.MetaAppID = strings.TrimSpace(in.MetaAppID)
	in.MetaAppSecret = strings.TrimSpace(in.MetaAppSecret)
	in.GoogleClientID = strings.TrimSpace(in.GoogleClientID)
	in.GoogleClientSecret = strings.TrimSpace(in.GoogleClientSecret)
	in.GoogleDeveloperToken = strings.TrimSpace(in.GoogleDeveloperToken)

	errs := map[string]string{}
	if in.Name == "" {
		errs["name"] = "required"
	}
	if !hexRe.MatchString(in.BrandColor) {
		errs["brandColor"] = "must be a hex color like #7c3aed"
	}
	if in.ContactEmail != "" {
		if _, err := mail.ParseAddress(in.ContactEmail); err != nil {
			errs["contactEmail"] = "invalid email"
		}
	}
	if len(errs) > 0 {
		return errs, nil
	}
	if err := s.repo.Update(ctx, id, in.Name, in.BrandColor, in.LogoURL, in.ContactEmail, in.Active, in.AvailabilitySlots, in.AvailabilityTimezone, in.GeminiAPIKey, in.MetaAppID, in.MetaAppSecret, in.GoogleClientID, in.GoogleClientSecret, in.GoogleDeveloperToken, in.SocialPlannerEnabled, in.KnowledgeBase, in.KnowledgeBaseFiles, in.TrialAmountSGD); err != nil {
		return nil, err
	}

	// Trigger RAG chunking if knowledge base, files, or key changed
	kbChanged := oldStudio == nil ||
		oldStudio.KnowledgeBase != in.KnowledgeBase ||
		oldStudio.GeminiAPIKey != in.GeminiAPIKey ||
		len(oldStudio.KnowledgeBaseFiles) != len(in.KnowledgeBaseFiles)

	if !kbChanged && oldStudio != nil {
		for idx, file := range oldStudio.KnowledgeBaseFiles {
			if file.Name != in.KnowledgeBaseFiles[idx].Name ||
				file.URL != in.KnowledgeBaseFiles[idx].URL ||
				file.Text != in.KnowledgeBaseFiles[idx].Text {
				kbChanged = true
				break
			}
		}
	}

	if kbChanged {
		s.AsyncSyncKnowledgeChunks(id, in.KnowledgeBase, in.KnowledgeBaseFiles, in.GeminiAPIKey)
	}

	return nil, nil
}

func (s *Service) AsyncSyncKnowledgeChunks(studioID uuid.UUID, kbText string, kbFiles []KnowledgeBaseFile, geminiAPIKey string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		apiKey := geminiAPIKey
		if apiKey == "" {
			var err error
			apiKey, err = s.repo.GetPlatformSetting(ctx, "gemini_api_key")
			if err != nil || apiKey == "" {
				// No Gemini API key available; skip chunk embedding generation
				return
			}
		}

		var rawChunks []struct {
			sourceType string
			sourceName string
			content    string
		}

		// 1. Chunk raw knowledge base text
		if kbText != "" {
			chunks := ChunkText(kbText, 800, 150)
			for _, content := range chunks {
				rawChunks = append(rawChunks, struct {
					sourceType string
					sourceName string
					content    string
				}{
					sourceType: "text",
					sourceName: "knowledge_base",
					content:    content,
				})
			}
		}

		// 2. Chunk knowledge base files
		for _, f := range kbFiles {
			if f.Text != "" {
				chunks := ChunkText(f.Text, 800, 150)
				for _, content := range chunks {
					rawChunks = append(rawChunks, struct {
						sourceType string
						sourceName string
						content    string
					}{
						sourceType: "file",
						sourceName: f.Name,
						content:    content,
					})
				}
			}
		}

		if len(rawChunks) == 0 {
			// Clear all chunks if no knowledge exists anymore
			_ = s.repo.SaveKnowledgeChunks(ctx, studioID, nil)
			return
		}

		var finalChunks []ChunkData
		for i, rc := range rawChunks {
			// Fetch embedding via Gemini API
			vec, err := GetGeminiEmbedding(ctx, apiKey, rc.content)
			if err != nil {
				// Continue to other chunks on transient error
				continue
			}
			finalChunks = append(finalChunks, ChunkData{
				SourceType: rc.sourceType,
				SourceName: rc.sourceName,
				Index:      i,
				Content:    rc.content,
				Embedding:  vec,
			})
			// Slight rate limit delay
			time.Sleep(100 * time.Millisecond)
		}

		if len(finalChunks) > 0 {
			_ = s.repo.SaveKnowledgeChunks(ctx, studioID, finalChunks)
		}
	}()
}


// ----- helpers -----

var (
	slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	hexRe  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func normalizeHex(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return strings.ToLower(s)
}

func generateSlug(name string) string {
	base := strings.ToLower(name)
	base = nonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "studio"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	return base
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func isPgUnique(err error) bool {
	type pgErr interface{ SQLState() string }
	var p pgErr
	if errors.As(err, &p) {
		return p.SQLState() == "23505"
	}
	return false
}

func (s *Service) UpdatePayments(ctx context.Context, id uuid.UUID, stripeAccountId, stripeSecretKey, stripePublishableKey, subscriptionTier string) error {
	return s.repo.UpdatePayments(ctx, id, stripeAccountId, stripeSecretKey, stripePublishableKey, subscriptionTier)
}

func (s *Service) ListPlans(ctx context.Context, studioID uuid.UUID) ([]Plan, error) {
	return s.repo.ListPlans(ctx, studioID)
}

func (s *Service) UpdatePlan(ctx context.Context, studioID, planID uuid.UUID, in UpdatePlanInput) error {
	return s.repo.UpdatePlan(ctx, studioID, planID, in)
}

func (s *Service) GetPlatformSetting(ctx context.Context, key string) (string, error) {
	return s.repo.GetPlatformSetting(ctx, key)
}

func (s *Service) UpdatePlatformSetting(ctx context.Context, key, value string) error {
	return s.repo.UpdatePlatformSetting(ctx, key, value)
}
