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
	Slug                 string
	Name                 string
	BrandColor           string
	LogoURL              string
	ContactEmail         string
	ContactPhone         string
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
	in.ContactPhone = strings.TrimSpace(in.ContactPhone)
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
	if in.ContactPhone != "" && len(in.ContactPhone) > 50 {
		errs["contactPhone"] = "must be under 50 characters"
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
		ContactPhone:         in.ContactPhone,
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
	ContactPhone         string `json:"contactPhone"`
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
	in.ContactPhone = strings.TrimSpace(in.ContactPhone)
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
	if in.ContactPhone != "" && len(in.ContactPhone) > 50 {
		errs["contactPhone"] = "must be under 50 characters"
	}
	if len(errs) > 0 {
		return errs, nil
	}
	if err := s.repo.Update(ctx, id, in.Name, in.BrandColor, in.LogoURL, in.ContactEmail, in.ContactPhone, in.Active, in.AvailabilitySlots, in.AvailabilityTimezone, in.GeminiAPIKey, in.MetaAppID, in.MetaAppSecret, in.GoogleClientID, in.GoogleClientSecret, in.GoogleDeveloperToken, in.SocialPlannerEnabled, in.KnowledgeBase, in.KnowledgeBaseFiles, in.TrialAmountSGD); err != nil {
		return nil, err
	}

	// Trigger RAG embedding rebuild if knowledge base or API key changed
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
		s.asyncSyncKnowledgeChunks(id, in.KnowledgeBase, in.KnowledgeBaseFiles, in.GeminiAPIKey)
	}

	return nil, nil
}

// asyncSyncKnowledgeChunks runs in a background goroutine so the HTTP
// response is not blocked by embedding API calls.
func (s *Service) asyncSyncKnowledgeChunks(studioID uuid.UUID, kbText string, kbFiles []KnowledgeBaseFile, geminiAPIKey string) {
	fmt.Printf("[RAG-DEBUG] asyncSyncKnowledgeChunks started for studio %s\n", studioID)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		apiKey := geminiAPIKey
		if apiKey == "" {
			if pk, err := s.repo.GetPlatformSetting(ctx, "gemini_api_key"); err == nil && pk != "" {
				apiKey = pk
			}
		}
		if apiKey == "" {
			fmt.Println("[RAG-DEBUG] No Gemini API key found. Skipping embedding sync.")
			return
		}
		fmt.Printf("[RAG-DEBUG] Resolved Gemini API Key (starts with %s)\n", apiKey[:8])

		type rawChunk struct{ sourceType, sourceName, content string }
		var raw []rawChunk

		for _, content := range ChunkText(kbText, 800, 150) {
			raw = append(raw, rawChunk{"text", "knowledge_base", content})
		}
		for _, f := range kbFiles {
			for _, content := range ChunkText(f.Text, 800, 150) {
				raw = append(raw, rawChunk{"file", f.Name, content})
			}
		}

		fmt.Printf("[RAG-DEBUG] Prepared %d raw chunks to process.\n", len(raw))

		if len(raw) == 0 {
			fmt.Println("[RAG-DEBUG] 0 chunks prepared. Clearing existing chunks.")
			err := s.repo.SaveKnowledgeChunks(ctx, studioID, nil)
			if err != nil {
				fmt.Printf("[RAG-DEBUG] SaveKnowledgeChunks error (clear): %v\n", err)
			}
			return
		}

		var chunks []ChunkData
		for i, rc := range raw {
			fmt.Printf("[RAG-DEBUG] Getting embedding for chunk %d (source: %s)...\n", i, rc.sourceName)
			vec, err := GetGeminiEmbedding(ctx, apiKey, rc.content)
			if err != nil {
				fmt.Printf("[RAG-DEBUG] GetGeminiEmbedding error on chunk %d: %v\n", i, err)
				continue
			}
			chunks = append(chunks, ChunkData{
				SourceType: rc.sourceType,
				SourceName: rc.sourceName,
				Index:      i,
				Content:    rc.content,
				Embedding:  vec,
			})
			time.Sleep(100 * time.Millisecond) // rate-limit courtesy
		}

		fmt.Printf("[RAG-DEBUG] Successfully generated %d embeddings out of %d chunks.\n", len(chunks), len(raw))

		if len(chunks) > 0 {
			err := s.repo.SaveKnowledgeChunks(ctx, studioID, chunks)
			if err != nil {
				fmt.Printf("[RAG-DEBUG] SaveKnowledgeChunks error: %v\n", err)
			} else {
				fmt.Println("[RAG-DEBUG] Successfully saved chunks and embeddings to database!")
			}
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

func (s *Service) UpdatePayments(ctx context.Context, id uuid.UUID, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier string) error {
	return s.repo.UpdatePayments(ctx, id, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier)
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
