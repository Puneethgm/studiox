package studios

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/integrations/crm"
	"github.com/projectx/api/internal/integrations/embeddings"
	"github.com/projectx/api/internal/integrations/glofox"
)

type Service struct {
	repo        *Repo
	identity    *identity.Repo
	glofox      *glofox.Client
	crmExecutor *crm.Executor
	embeddings  *embeddings.Client
}

func NewService(repo *Repo, id *identity.Repo, gf *glofox.Client, crmExecutor *crm.Executor, embClient *embeddings.Client) *Service {
	return &Service{repo: repo, identity: id, glofox: gf, crmExecutor: crmExecutor, embeddings: embClient}
}

// SyncLeadToGlofoxByID pushes a lead to Glofox CRM as trial/member. Used by
// the Stripe webhook handler, which confirms actual payment (checkout.session.
// completed) and updates the lead via raw SQL rather than through leads.Repo,
// so it needs its own path to Glofox rather than relying on leads.Repo's sync.
// amountCents is what Stripe actually charged (its smallest-unit convention),
// used to auto-match a Glofox membership/plan by price when the studio hasn't
// manually mapped one.
func (s *Service) SyncLeadToGlofoxByID(ctx context.Context, leadID string, status glofox.GlofoxLeadStatus, amountCents int64) {
	if s.glofox == nil || leadID == "" {
		return
	}
	var firstName, lastName, name, email, phone, source, studioIDStr string
	var gender string
	var dateOfBirth *time.Time
	err := s.repo.Pool().QueryRow(ctx, `
		SELECT COALESCE(first_name,''), COALESCE(last_name,''), COALESCE(name,''), COALESCE(email,''), COALESCE(phone,''), COALESCE(source,''), studio_id::text, COALESCE(gender,''), date_of_birth
		FROM leads WHERE id = $1
	`, leadID).Scan(&firstName, &lastName, &name, &email, &phone, &source, &studioIDStr, &gender, &dateOfBirth)
	if err != nil {
		slog.Warn("Glofox | Lead sync: failed to load lead for sync", "component", "glofox", "lead_id", leadID, "err", err.Error())
		return
	}
	birthDate := ""
	if dateOfBirth != nil {
		birthDate = dateOfBirth.Format("2006-01-02")
	}
	if firstName == "" && lastName == "" && name != "" {
		parts := strings.SplitN(name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}
	if lastName == "" {
		// Glofox rejects the request outright without a last name — many
		// WhatsApp leads only ever give a first name.
		lastName = "-"
	}

	// Look up the studio's Glofox membership/plan-code mapping (if any) so
	// we can also record an actual credit-pack/membership purchase, not
	// just a bare lead record.
	var membershipID, planCode string
	if studioID, parseErr := uuid.Parse(studioIDStr); parseErr == nil {
		if studio, sErr := s.repo.GetByID(ctx, studioID); sErr == nil && studio != nil {
			if status == glofox.GlofoxStatusMember {
				membershipID = studio.MembershipGlofoxMembershipID
				planCode = studio.MembershipGlofoxPlanCode
			} else {
				membershipID = studio.TrialGlofoxMembershipID
				planCode = studio.TrialGlofoxPlanCode
			}
		}
	}

	go func() {
		gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := s.glofox.CreateLead(gCtx, glofox.CreateLeadInput{
			Email:         email,
			FirstName:     firstName,
			LastName:      lastName,
			Phone:         phone,
			LeadStatus:    status,
			ContactSource: source,
			Gender:        gender,
			BirthDate:     birthDate,
		})
		if err != nil {
			slog.Warn("Glofox | Lead sync failed — lead conversion not reflected in Glofox CRM",
				"component", "glofox", "lead_id", leadID, "new_status", string(status), "error", err.Error())
			return
		}
		slog.Info("Glofox | Lead synced to Glofox CRM",
			"component", "glofox", "lead_id", leadID, "new_status", string(status), "glofox_id", out.Entity.ID)

		if (membershipID == "" || planCode == "") && amountCents > 0 {
			match, mErr := s.glofox.FindMembershipPlanByPrice(gCtx, amountCents, status == glofox.GlofoxStatusTrial)
			if mErr != nil {
				slog.Warn("Glofox | Could not auto-match a membership plan for this payment — no purchase recorded",
					"component", "glofox", "lead_id", leadID, "amount_cents", amountCents, "error", mErr.Error())
			} else {
				membershipID, planCode = match.MembershipID, match.PlanCode
			}
		}
		if membershipID == "" || planCode == "" {
			return
		}
		purchase, pErr := s.glofox.PurchaseMembership(gCtx, glofox.PurchaseMembershipInput{
			UserID:        out.Entity.ID,
			MembershipID:  membershipID,
			PlanCode:      planCode,
			PaymentMethod: "cash",
		})
		if pErr != nil {
			slog.Warn("Glofox | Membership purchase failed — lead created but no credit-pack/membership recorded",
				"component", "glofox", "lead_id", leadID, "membership_id", membershipID, "plan_code", planCode, "error", pErr.Error())
			return
		}
		slog.Info("Glofox | Membership purchase recorded",
			"component", "glofox", "lead_id", leadID, "invoice_id", purchase.InvoiceID, "status", purchase.Status)
		if purchase.InvoiceID != "" {
			_, _ = s.repo.Pool().Exec(gCtx, `UPDATE leads SET glofox_invoice_id = $2 WHERE id = $1`, leadID, purchase.InvoiceID)
		}
	}()
}

// SyncLeadToMindbodyByID pushes a lead to Mindbody via the generic CRM
// framework's create_lead operation (crm.Executor + a per-studio
// crm_connections row), then looks up Mindbody's service pricing to log
// which plan this payment likely corresponds to. Mirrors
// SyncLeadToGlofoxByID's shape but goes through the data-driven executor
// rather than a hardcoded client, since Mindbody is onboarded through the
// new provider/operation/connection framework (internal/integrations/crm)
// instead of a dedicated Go client like glofox.Client.
//
// Recording the actual sale (Mindbody's CheckoutShoppingCart) is
// deliberately not done here: that endpoint requires staff-level
// (login-then-bearer) auth, which crm.Executor.applyAuth doesn't support
// yet, and no working staff credential has been available to build or
// verify it against. Only create-lead + pricing lookup are wired.
func (s *Service) SyncLeadToMindbodyByID(ctx context.Context, leadID string, isTrial bool, amountCents int64) {
	if s.crmExecutor == nil || leadID == "" {
		return
	}
	var firstName, lastName, name, email, phone, studioIDStr string
	var dateOfBirth *time.Time
	err := s.repo.Pool().QueryRow(ctx, `
		SELECT COALESCE(first_name,''), COALESCE(last_name,''), COALESCE(name,''), COALESCE(email,''), COALESCE(phone,''), studio_id::text, date_of_birth
		FROM leads WHERE id = $1
	`, leadID).Scan(&firstName, &lastName, &name, &email, &phone, &studioIDStr, &dateOfBirth)
	if err != nil {
		slog.Warn("Mindbody | Lead sync: failed to load lead for sync", "component", "mindbody", "lead_id", leadID, "err", err.Error())
		return
	}
	if firstName == "" && lastName == "" && name != "" {
		parts := strings.SplitN(name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}
	if lastName == "" {
		lastName = "-"
	}
	birthDate := "1990-01-01" // neutral default — Mindbody requires one, we don't always collect it
	if dateOfBirth != nil {
		birthDate = dateOfBirth.Format("2006-01-02")
	}
	studioID, err := uuid.Parse(studioIDStr)
	if err != nil {
		return
	}

	go func() {
		mCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := s.crmExecutor.Execute(mCtx, studioID, crm.OpCreateLead, map[string]any{
			"email":      email,
			"firstName":  firstName,
			"lastName":   lastName,
			"phone":      phone,
			"birthDate":  birthDate,
			"street":     "N/A",
			"city":       "N/A",
			"state":      "NA",
			"postalCode": "00000",
			"referredBy": "Project-X",
		})
		if err != nil {
			// Most studios won't have a Mindbody connection configured —
			// that's the expected, silent path for all of them, same as
			// Glofox's own "not configured" no-op above.
			slog.Debug("Mindbody | Lead sync skipped or failed", "component", "mindbody", "lead_id", leadID, "err", err.Error())
			return
		}
		mindbodyID, _ := result["userId"].(string)
		slog.Info("Mindbody | Lead synced to Mindbody CRM", "component", "mindbody", "lead_id", leadID, "mindbody_client_id", mindbodyID)

		if amountCents <= 0 {
			return
		}
		planResult, err := s.crmExecutor.Execute(mCtx, studioID, crm.OpFindMembershipPlan, map[string]any{
			"amountCents": amountCents,
			"isTrial":     isTrial,
		})
		if err != nil {
			slog.Warn("Mindbody | Plan/pricing lookup failed", "component", "mindbody", "lead_id", leadID, "err", err.Error())
			return
		}
		services, _ := planResult["Services"].([]any)
		target := float64(amountCents) / 100.0
		var matchedName string
		for _, raw := range services {
			svc, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if price, _ := svc["Price"].(float64); price == target {
				matchedName, _ = svc["Name"].(string)
				break
			}
		}
		if matchedName != "" {
			slog.Info("Mindbody | Matching service/plan found for this payment", "component", "mindbody", "lead_id", leadID, "amount_cents", amountCents, "matched_service", matchedName)
		} else {
			slog.Info("Mindbody | No exact-price service match found — payment logged without a plan match", "component", "mindbody", "lead_id", leadID, "amount_cents", amountCents)
		}
	}()
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
		ManagedBy1Hero:       true,
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

	// Register the studio admin in Glofox asynchronously.
	// Failure here must never block or roll back studio creation.
	if s.glofox != nil {
		firstName, lastName := splitName(in.Name)
		studioName := studio.Name
		adminEmail := in.AdminEmail
		studioID := studio.ID
		go func() {
			gCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := s.glofox.RegisterUser(gCtx, glofox.RegisterUserInput{
				Email:     adminEmail,
				FirstName: firstName,
				LastName:  lastName,
				Phone:     in.ContactPhone,
				Password:  in.AdminPassword,
			})
			if err != nil {
				slog.Warn("Glofox | Studio admin account creation failed — admin will not appear in Glofox CRM",
					"component", "glofox",
					"studio_id", studioID,
					"studio_name", studioName,
					"admin_email", adminEmail,
					"error", err.Error(),
				)
			} else {
				slog.Info("Glofox | Studio admin account created successfully",
					"component", "glofox",
					"studio_id", studioID,
					"studio_name", studioName,
					"admin_email", adminEmail,
				)
			}
		}()
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
	Name                          string
	BrandColor                    string
	LogoURL                       string
	ContactEmail                  string
	ContactPhone                  string `json:"contactPhone"`
	Active                        bool
	ManagedBy1Hero                bool
	AvailabilitySlots             []AvailabilitySlot  `json:"availabilitySlots"`
	AvailabilityTimezone          string              `json:"availabilityTimezone"`
	GeminiAPIKey                  string              `json:"geminiApiKey"`
	GroqAPIKey                    string              `json:"groqApiKey"`
	MetaAppID                     string              `json:"metaAppId"`
	MetaAppSecret                 string              `json:"metaAppSecret"`
	GoogleClientID                string              `json:"googleClientId"`
	GoogleClientSecret            string              `json:"googleClientSecret"`
	GoogleDeveloperToken          string              `json:"googleDeveloperToken"`
	SocialPlannerEnabled          bool                `json:"socialPlannerEnabled"`
	KnowledgeBase                 string              `json:"knowledgeBase"`
	KnowledgeBaseFiles            []KnowledgeBaseFile `json:"knowledgeBaseFiles"`
	GreetingMessage               string              `json:"greetingMessage"`
	TrialAmountSGD                int                 `json:"trialAmountSgd"`
	BookingHeroImageURL           string              `json:"bookingHeroImageUrl"`
	BookingHeroVideoURL           string              `json:"bookingHeroVideoUrl"`
	TrialConfirmationMessage      string              `json:"trialConfirmationMessage"`
	MembershipConfirmationMessage string              `json:"membershipConfirmationMessage"`
	TrialGlofoxMembershipID       string              `json:"trialGlofoxMembershipId"`
	TrialGlofoxPlanCode           string              `json:"trialGlofoxPlanCode"`
	MembershipGlofoxMembershipID  string              `json:"membershipGlofoxMembershipId"`
	MembershipGlofoxPlanCode      string              `json:"membershipGlofoxPlanCode"`
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
	if err := s.repo.Update(ctx, id, in.Name, in.BrandColor, in.LogoURL, in.ContactEmail, in.ContactPhone, in.Active, in.ManagedBy1Hero, in.AvailabilitySlots, in.AvailabilityTimezone, in.GeminiAPIKey, in.GroqAPIKey, in.MetaAppID, in.MetaAppSecret, in.GoogleClientID, in.GoogleClientSecret, in.GoogleDeveloperToken, in.SocialPlannerEnabled, in.KnowledgeBase, in.KnowledgeBaseFiles, in.GreetingMessage, in.TrialAmountSGD, in.BookingHeroImageURL, in.BookingHeroVideoURL, in.TrialConfirmationMessage, in.MembershipConfirmationMessage, in.TrialGlofoxMembershipID, in.TrialGlofoxPlanCode, in.MembershipGlofoxMembershipID, in.MembershipGlofoxPlanCode); err != nil {
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
		s.asyncSyncKnowledgeChunks(id, in.KnowledgeBase, in.KnowledgeBaseFiles)
	}

	return nil, nil
}

// asyncSyncKnowledgeChunks runs in a background goroutine so the HTTP
// response is not blocked by embedding API calls.
func (s *Service) asyncSyncKnowledgeChunks(studioID uuid.UUID, kbText string, kbFiles []KnowledgeBaseFile) {
	go func() {
		// Raised from 5min to 30min to cover very large documents (up to
		// 200MB uploads) — a heavily text-dense file can chunk into tens of
		// thousands of pieces, each embedded sequentially with a rate-limit
		// sleep (see the loop below); 5 minutes wasn't enough headroom and
		// chunks past the deadline were silently dropped rather than synced.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Status is polled by the Knowledge Base admin page so it can tell the
		// admin when a just-uploaded document is actually searchable, instead
		// of the misleading impression that saving the file means it's done —
		// this goroutine can run for a while after the HTTP response returns.
		if err := s.repo.SetKnowledgeSyncStatus(ctx, studioID, "syncing"); err != nil {
			slog.Warn("set knowledge sync status failed", "studio_id", studioID, "err", err)
		}

		// Program-schedule parsing is independent of the embeddings service
		// (it's a plain-text regex match, not a vector search) and of the
		// KB-text chunking below — run it first so a studio without
		// embeddings configured still gets exact "today's session" lookups.
		// Runs across the main KB text and every uploaded file; a document
		// that doesn't match the week-by-week format just yields nothing
		// (see ParseProgramSchedule's doc comment), so this is a no-op for
		// the vast majority of knowledge base uploads.
		var programEntries []ProgramSessionEntry
		programEntries = append(programEntries, ParseProgramSchedule(kbText)...)
		for _, f := range kbFiles {
			programEntries = append(programEntries, ParseProgramSchedule(f.Text)...)
		}
		if err := s.repo.SaveProgramSessions(ctx, studioID, programEntries); err != nil {
			slog.Error("save program sessions failed", "studio_id", studioID, "err", err)
		} else if len(programEntries) > 0 {
			slog.Info("parsed program schedule", "studio_id", studioID, "sessions", len(programEntries))
		}

		if s.embeddings == nil {
			slog.Warn("knowledge sync skipped: embeddings service not configured", "studio_id", studioID)
			_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "error")
			return
		}

		type rawChunk struct{ sourceType, sourceName, platform, content string }
		var raw []rawChunk

		// Main text always applies to every platform.
		for _, content := range ChunkText(kbText, 800, 150) {
			raw = append(raw, rawChunk{"text", "knowledge_base", "all", content})
		}
		for _, f := range kbFiles {
			platform := f.Platform
			if platform == "" {
				platform = "all"
			}
			for _, content := range ChunkText(f.Text, 800, 150) {
				raw = append(raw, rawChunk{"file", f.Name, platform, content})
			}
		}

		if len(raw) == 0 {
			if err := s.repo.SaveKnowledgeChunks(ctx, studioID, nil); err != nil {
				slog.Error("knowledge chunks clear failed", "studio_id", studioID, "err", err)
				_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "error")
				return
			}
			_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "complete")
			return
		}

		var chunks []ChunkData
		for i, rc := range raw {
			vec, err := s.embeddings.EmbedPassage(ctx, rc.content)
			if err != nil {
				slog.Warn("embedding failed for chunk", "studio_id", studioID, "chunk", i, "err", err)
				continue
			}
			chunks = append(chunks, ChunkData{
				SourceType: rc.sourceType,
				SourceName: rc.sourceName,
				Platform:   rc.platform,
				Index:      i,
				Content:    rc.content,
				Embedding:  vec,
			})
			time.Sleep(100 * time.Millisecond)
		}

		if len(chunks) == 0 {
			slog.Error("knowledge sync produced no chunks", "studio_id", studioID, "attempted", len(raw))
			_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "error")
			return
		}

		if err := s.repo.SaveKnowledgeChunks(ctx, studioID, chunks); err != nil {
			slog.Error("knowledge chunks save failed", "studio_id", studioID, "err", err)
			_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "error")
			return
		}
		slog.Info("knowledge sync complete", "studio_id", studioID, "chunks", len(chunks))
		_ = s.repo.SetKnowledgeSyncStatus(ctx, studioID, "complete")
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

// splitName splits a full name string into first and last components.
// If there is only one word it is used as firstName with lastName empty.
func splitName(name string) (first, last string) {
	name = strings.TrimSpace(name)
	idx := strings.LastIndex(name, " ")
	if idx < 0 {
		return name, ""
	}
	return strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+1:])
}

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

// ResolveTrialAmountSGD is the single source of truth for "what does this
// studio's trial actually cost, in cents" — studio.trial_amount_sgd override
// → lowest active Plan price → 2500 (S$25) fallback. Used both by the public
// trial-details page (so the price shown matches what Stripe will charge)
// and by trial payment-intent/checkout creation. Don't duplicate this
// resolution elsewhere — call this instead.
func (s *Service) ResolveTrialAmountSGD(ctx context.Context, studioID uuid.UUID, trialAmountSGD int) int64 {
	amount := int64(trialAmountSGD)
	if amount == 0 {
		plans, _ := s.ListPlans(ctx, studioID)
		for _, p := range plans {
			if !p.IsActive {
				continue
			}
			if amount == 0 || int64(p.PriceSGD) < amount {
				amount = int64(p.PriceSGD)
			}
		}
	}
	if amount == 0 {
		amount = 2500
	}
	return amount
}

func (s *Service) CreatePlan(ctx context.Context, studioID uuid.UUID, in CreatePlanInput) (Plan, error) {
	return s.repo.CreatePlan(ctx, studioID, in)
}

func (s *Service) UpdatePlan(ctx context.Context, studioID, planID uuid.UUID, in UpdatePlanInput) error {
	return s.repo.UpdatePlan(ctx, studioID, planID, in)
}

func (s *Service) DeletePlan(ctx context.Context, studioID, planID uuid.UUID) error {
	return s.repo.DeletePlan(ctx, studioID, planID)
}

func (s *Service) GetPlatformSetting(ctx context.Context, key string) (string, error) {
	return s.repo.GetPlatformSetting(ctx, key)
}

func (s *Service) UpdatePlatformSetting(ctx context.Context, key, value string) error {
	return s.repo.UpdatePlatformSetting(ctx, key, value)
}
