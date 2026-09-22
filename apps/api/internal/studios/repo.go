package studios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/cache"
	"github.com/projectx/api/internal/platform/secrets"
)

type Repo struct {
	pool   *pgxpool.Pool
	cache  *cache.MemoryCache
	cipher *secrets.Cipher
}

func NewRepo(pool *pgxpool.Pool, cipher *secrets.Cipher) *Repo {
	return &Repo{
		pool:   pool,
		cache:  cache.New(),
		cipher: cipher,
	}
}

// Pool exposes the underlying pool so the studios service can run a
// transactional create-studio-with-admin flow.
func (r *Repo) Pool() *pgxpool.Pool { return r.pool }

func (r *Repo) Create(ctx context.Context, tx pgx.Tx, s *Studio) error {
	var err error
	encWebhookSecret := s.StripeWebhookSecret
	if encWebhookSecret != "" && r.cipher != nil {
		encWebhookSecret, err = r.cipher.Encrypt(encWebhookSecret)
		if err != nil {
			return fmt.Errorf("encrypt stripe webhook secret: %w", err)
		}
	}
	kbFilesJSON, _ := json.Marshal(s.KnowledgeBaseFiles)
	row := tx.QueryRow(ctx, `
		INSERT INTO studios (slug, name, brand_color, logo_url, contact_email, contact_phone, active, gemini_api_key, groq_api_key, claude_api_key, meta_app_id, meta_app_secret, google_client_id, google_client_secret, google_developer_token, stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files, trial_amount_sgd, managed_by_1hero)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25)
		RETURNING id, created_at, updated_at
	`, s.Slug, s.Name, s.BrandColor, s.LogoURL, s.ContactEmail, s.ContactPhone, s.Active, s.GeminiAPIKey, s.GroqAPIKey, s.ClaudeAPIKey, s.MetaAppID, s.MetaAppSecret, s.GoogleClientID, s.GoogleClientSecret, s.GoogleDeveloperToken, s.StripeAccountID, s.StripeSecretKey, s.StripePublishableKey, encWebhookSecret, s.SubscriptionTier, s.SocialPlannerEnabled, s.KnowledgeBase, string(kbFilesJSON), s.TrialAmountSGD, s.ManagedBy1Hero)
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
		SELECT s.id, s.slug, s.name, s.brand_color, s.logo_url, s.contact_email, s.contact_phone,
		       s.active, s.created_at, s.updated_at, s.availability_slots, s.availability_timezone, s.gemini_api_key, s.meta_app_id, s.meta_app_secret,
		       s.google_client_id, s.google_client_secret, s.google_developer_token,
		       s.stripe_account_id, s.stripe_secret_key, s.stripe_publishable_key, s.stripe_webhook_secret, s.subscription_tier, s.social_planner_enabled, s.knowledge_base, s.knowledge_base_files,
		       s.trial_amount_sgd, s.managed_by_1hero,
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
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.BrandColor, &s.LogoURL, &s.ContactEmail, &s.ContactPhone,
			&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.GeminiAPIKey, &s.MetaAppID, &s.MetaAppSecret,
			&s.GoogleClientID, &s.GoogleClientSecret, &s.GoogleDeveloperToken,
			&s.StripeAccountID, &s.StripeSecretKey, &s.StripePublishableKey, &s.StripeWebhookSecret, &s.SubscriptionTier, &s.SocialPlannerEnabled, &s.KnowledgeBase, &s.KnowledgeBaseFiles,
			&s.TrialAmountSGD, &s.ManagedBy1Hero, &s.CampaignCount, &s.LeadCount); err != nil {
			return nil, fmt.Errorf("scan studio: %w", err)
		}
		if s.StripeSecretKey != "" && r.cipher != nil {
			dec, err := r.cipher.Decrypt(s.StripeSecretKey)
			if err == nil {
				s.StripeSecretKey = dec
			}
		}
		if s.StripeWebhookSecret != "" && r.cipher != nil {
			dec, err := r.cipher.Decrypt(s.StripeWebhookSecret)
			if err == nil {
				s.StripeWebhookSecret = dec
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Studio, error) {
	key := "studio:id:" + id.String()
	if val, ok := r.cache.Get(key); ok {
		if s, ok := val.(*Studio); ok {
			return s, nil
		}
	}

	row := r.pool.QueryRow(ctx, `
		SELECT id, slug, name, brand_color, logo_url, contact_email, contact_phone, active, created_at, updated_at,
		       availability_slots, availability_timezone, gemini_api_key, groq_api_key, claude_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url,
		       trial_confirmation_message, membership_confirmation_message,
		       trial_glofox_membership_id, trial_glofox_plan_code, membership_glofox_membership_id, membership_glofox_plan_code,
		       communication_style_profile, style_profile_updated_at, style_refresh_interval_minutes, program_start_date
		FROM studios WHERE id = $1
	`, id)
	s, err := scanStudio(row, r.cipher)
	if err == nil {
		r.cache.Set(key, s, 10*time.Minute)
		r.cache.Set("studio:slug:"+s.Slug, s, 10*time.Minute)
	}
	return s, err
}

func (r *Repo) GetBySlug(ctx context.Context, slug string) (*Studio, error) {
	key := "studio:slug:" + slug
	if val, ok := r.cache.Get(key); ok {
		if s, ok := val.(*Studio); ok {
			return s, nil
		}
	}

	row := r.pool.QueryRow(ctx, `
		SELECT id, slug, name, brand_color, logo_url, contact_email, contact_phone, active, created_at, updated_at,
		       availability_slots, availability_timezone, gemini_api_key, groq_api_key, claude_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url,
		       trial_confirmation_message, membership_confirmation_message,
		       trial_glofox_membership_id, trial_glofox_plan_code, membership_glofox_membership_id, membership_glofox_plan_code,
		       communication_style_profile, style_profile_updated_at, style_refresh_interval_minutes, program_start_date
		FROM studios WHERE slug = $1
	`, slug)
	s, err := scanStudio(row, r.cipher)
	if err == nil {
		r.cache.Set(key, s, 10*time.Minute)
		r.cache.Set("studio:id:"+s.ID.String(), s, 10*time.Minute)
	}
	return s, err
}

// Update writes the editable fields. Slug is intentionally NOT updatable here
// (changing a slug breaks every shared public link). Add a deliberate "rename
// slug" flow when needed.
func (r *Repo) Update(ctx context.Context, id uuid.UUID, name, brandColor, logoURL, contactEmail, contactPhone string, active bool, managedBy1Hero bool, availabilitySlots []AvailabilitySlot, availabilityTimezone string, geminiAPIKey, groqAPIKey, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken string, socialPlannerEnabled bool, knowledgeBase string, knowledgeBaseFiles []KnowledgeBaseFile, greetingMessage string, trialAmountSGD int, bookingHeroImageURL, bookingHeroVideoURL string, trialConfirmationMessage, membershipConfirmationMessage string, trialGlofoxMembershipID, trialGlofoxPlanCode, membershipGlofoxMembershipID, membershipGlofoxPlanCode string) error {
	slotsJSON, _ := json.Marshal(availabilitySlots)
	filesJSON, _ := json.Marshal(knowledgeBaseFiles)
	tag, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET name = $2, brand_color = $3, logo_url = $4, contact_email = $5, contact_phone = $6, active = $7,
		    availability_slots = $8, availability_timezone = $9, gemini_api_key = $10, groq_api_key = $11,
		    meta_app_id = $12, meta_app_secret = $13, google_client_id = $14, google_client_secret = $15, google_developer_token = $16,
		    social_planner_enabled = $17, knowledge_base = $18, knowledge_base_files = $19,
		    greeting_message = $20, trial_amount_sgd = $21, managed_by_1hero = $22, booking_hero_image_url = $23, booking_hero_video_url = $24,
		    trial_confirmation_message = $25, membership_confirmation_message = $26,
		    trial_glofox_membership_id = $27, trial_glofox_plan_code = $28, membership_glofox_membership_id = $29, membership_glofox_plan_code = $30, updated_at = now()
		WHERE id = $1`,
		id, name, brandColor, logoURL, contactEmail, contactPhone, active, string(slotsJSON), availabilityTimezone, geminiAPIKey, groqAPIKey, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken, socialPlannerEnabled, knowledgeBase, string(filesJSON), greetingMessage, trialAmountSGD, managedBy1Hero, bookingHeroImageURL, bookingHeroVideoURL, trialConfirmationMessage, membershipConfirmationMessage, trialGlofoxMembershipID, trialGlofoxPlanCode, membershipGlofoxMembershipID, membershipGlofoxPlanCode)
	if err != nil {
		return fmt.Errorf("update studio: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Cache eviction
	r.evict(id)
	return nil
}

// GetTrialPageLayout returns a studio's saved trial-payment-page block
// layout, or nil if it has never customized one (the frontend falls back to
// a sensible built-in default in that case). Deliberately NOT part of
// GetByID/GetBySlug's cached Studio struct — this is edited from its own
// dedicated builder page and must reflect a save immediately, not wait out
// the 10-minute studio cache TTL.
func (r *Repo) GetTrialPageLayout(ctx context.Context, studioID uuid.UUID) (json.RawMessage, error) {
	var layout []byte
	err := r.pool.QueryRow(ctx, `SELECT trial_page_layout FROM studios WHERE id = $1`, studioID).Scan(&layout)
	if err != nil {
		return nil, err
	}
	return layout, nil
}

// GetTrialPageLayoutBySlug is the public-read counterpart, used by the
// customer-facing trial payment page.
func (r *Repo) GetTrialPageLayoutBySlug(ctx context.Context, slug string) (json.RawMessage, error) {
	var layout []byte
	err := r.pool.QueryRow(ctx, `SELECT trial_page_layout FROM studios WHERE slug = $1`, slug).Scan(&layout)
	if err != nil {
		return nil, err
	}
	return layout, nil
}

func (r *Repo) SetTrialPageLayout(ctx context.Context, studioID uuid.UUID, layout json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET trial_page_layout = $2, updated_at = now() WHERE id = $1
	`, studioID, layout)
	return err
}

// GetInitialContactDelayMinutes returns how long the autocontact worker
// should wait before sending a newly-created lead's initial WhatsApp
// message. Read fresh on every call (not cached) since it's cheap and
// avoids invalidation coupling with the GetByID studio cache.
func (r *Repo) GetInitialContactDelayMinutes(ctx context.Context, studioID uuid.UUID) (int, error) {
	var minutes int
	err := r.pool.QueryRow(ctx, `
		SELECT initial_contact_delay_minutes FROM studios WHERE id = $1
	`, studioID).Scan(&minutes)
	if err != nil {
		return 0, err
	}
	return minutes, nil
}

func (r *Repo) SetInitialContactDelayMinutes(ctx context.Context, studioID uuid.UUID, minutes int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET initial_contact_delay_minutes = $2, updated_at = now() WHERE id = $1
	`, studioID, minutes)
	return err
}

// GetAIReplyDelaySeconds returns how long the AI worker should wait before
// sending its auto-reply to an inbound conversation message (0 = immediate).
// A brief delay reads as more human than an instant reply. Doesn't apply to
// the very first outreach to a lead — see initial_contact_delay_minutes for
// that — only to replies within an already-started conversation.
func (r *Repo) GetAIReplyDelaySeconds(ctx context.Context, studioID uuid.UUID) (int, error) {
	var seconds int
	err := r.pool.QueryRow(ctx, `
		SELECT ai_reply_delay_seconds FROM studios WHERE id = $1
	`, studioID).Scan(&seconds)
	if err != nil {
		return 0, err
	}
	return seconds, nil
}

func (r *Repo) SetAIReplyDelaySeconds(ctx context.Context, studioID uuid.UUID, seconds int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET ai_reply_delay_seconds = $2, updated_at = now() WHERE id = $1
	`, studioID, seconds)
	return err
}

// IncrementStaffReplyCount bumps the running counter of staff-authored
// (source_kind='studio_user') outbound messages sent for a studio. Called
// once per staff-sent message by the outbound worker. Deliberately not part
// of the cached Studio struct — it's read only via ListStudiosNeedingStyleRefresh,
// never displayed, so there's no cache-staleness concern to manage here.
func (r *Repo) IncrementStaffReplyCount(ctx context.Context, studioID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET staff_reply_count_total = staff_reply_count_total + 1 WHERE id = $1
	`, studioID)
	return err
}

// SetStaffReplyCountTotal overwrites the counter with an exact value, used
// to reconcile it after a WhatsApp/Telegram Web backfill import inserts a
// batch of historical staff replies directly (bypassing the per-send
// IncrementStaffReplyCount call), so those newly-visible replies count
// toward the next style-profile rebuild instead of being invisible to it.
func (r *Repo) SetStaffReplyCountTotal(ctx context.Context, studioID uuid.UUID, count int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET staff_reply_count_total = $2 WHERE id = $1
	`, studioID, count)
	return err
}

// SetStyleRefreshIntervalMinutes saves how often (at minimum) a studio
// wants its communication style profile re-learned — see
// ListStudiosNeedingStyleRefresh for how this combines with the
// new-replies threshold. No dedicated getter: the value already comes back
// in the general studio response (studioResponse), same as
// communicationStyleProfile.
func (r *Repo) SetStyleRefreshIntervalMinutes(ctx context.Context, studioID uuid.UUID, minutes int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET style_refresh_interval_minutes = $2, updated_at = now() WHERE id = $1
	`, studioID, minutes)
	if err != nil {
		return err
	}
	// Unlike initial_contact_delay_minutes (deliberately outside the cached
	// Studio struct), this column IS part of GetByID/GetBySlug's SELECT —
	// evict so a save reflects immediately instead of waiting out the
	// 10-minute cache TTL.
	r.evict(studioID)
	return nil
}

// SetProgramStartDate anchors a parsed week-by-week program document (see
// ParseProgramSchedule) to a real calendar date — Week 1's first day.
func (r *Repo) SetProgramStartDate(ctx context.Context, studioID uuid.UUID, date time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET program_start_date = $2, updated_at = now() WHERE id = $1
	`, studioID, date)
	if err != nil {
		return err
	}
	r.evict(studioID)
	return nil
}

// ClearProgramStartDate unsets the program anchor date — the AI worker's
// deterministic lookup (see buildPrompt) is then simply skipped, falling
// back to the general week-arithmetic instruction.
func (r *Repo) ClearProgramStartDate(ctx context.Context, studioID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET program_start_date = NULL, updated_at = now() WHERE id = $1
	`, studioID)
	if err != nil {
		return err
	}
	r.evict(studioID)
	return nil
}

// SaveProgramSessions atomically replaces every parsed program-schedule
// entry for a studio — same replace-all-on-resync pattern as
// SaveKnowledgeChunks, so a re-upload/edit of the source document can't
// leave stale sessions from a previous version mixed in.
func (r *Repo) SaveProgramSessions(ctx context.Context, studioID uuid.UUID, entries []ProgramSessionEntry) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM studio_program_sessions WHERE studio_id = $1`, studioID); err != nil {
		return fmt.Errorf("delete old program sessions: %w", err)
	}
	for _, e := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO studio_program_sessions
				(studio_id, week_number, day_of_week, day_name, session_name, progression, key_focus)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, studioID, e.WeekNumber, e.DayOfWeek, e.DayName, e.SessionName, e.Progression, e.KeyFocus); err != nil {
			return fmt.Errorf("insert program session: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// GetProgramSession looks up the exact parsed entry for a given week/day —
// a real database lookup, not a semantic-search guess, so it can't return
// the wrong week the way RAG retrieval did (see ParseProgramSchedule's
// doc comment). Returns nil, nil if nothing is on file for that day (e.g.
// dayOfWeek is a rest day the program doesn't list, or no schedule document
// has been parsed for this studio at all).
func (r *Repo) GetProgramSession(ctx context.Context, studioID uuid.UUID, weekNumber, dayOfWeek int) (*ProgramSessionEntry, error) {
	var e ProgramSessionEntry
	err := r.pool.QueryRow(ctx, `
		SELECT week_number, day_of_week, day_name, session_name, progression, key_focus
		FROM studio_program_sessions
		WHERE studio_id = $1 AND week_number = $2 AND day_of_week = $3
	`, studioID, weekNumber, dayOfWeek).Scan(
		&e.WeekNumber, &e.DayOfWeek, &e.DayName, &e.SessionName, &e.Progression, &e.KeyFocus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get program session: %w", err)
	}
	return &e, nil
}

// ListStudiosNeedingStyleRefresh returns studio IDs due for a communication
// style rebuild — either because they've accumulated at least `threshold`
// new staff replies since the profile was last built (the original,
// volume-based trigger), OR because their own configurable
// style_refresh_interval_minutes has elapsed since the last build AND at
// least one new reply has arrived (so a quiet studio with zero new
// material never wastes an LLM call re-producing the same profile on a
// timer).
func (r *Repo) ListStudiosNeedingStyleRefresh(ctx context.Context, threshold int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM studios
		WHERE staff_reply_count_total - style_profile_source_count >= $1
		   OR (
		        staff_reply_count_total - style_profile_source_count > 0
		        AND now() - COALESCE(style_profile_updated_at, created_at)
		            >= (style_refresh_interval_minutes || ' minutes')::interval
		      )
	`, threshold)
	if err != nil {
		return nil, fmt.Errorf("list studios needing style refresh: %w", err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan studio id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetCommunicationStyleProfile persists a (re)built style profile and marks
// the source counter it was built from — so the next refresh only fires
// once *another* `threshold` worth of new staff replies have arrived. Used
// both by the style worker's automatic rebuilds and by a studio admin's
// manual edit (an edit "confirms" the current text, which is exactly the
// same reset the worker itself performs after a rebuild).
func (r *Repo) SetCommunicationStyleProfile(ctx context.Context, studioID uuid.UUID, profile string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET communication_style_profile = $2,
		    style_profile_source_count = staff_reply_count_total,
		    style_profile_updated_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, studioID, profile)
	if err != nil {
		return err
	}
	r.evict(studioID)
	return nil
}

// aiProviderKeyColumns whitelists which column UpdateAIProviderKey may write
// to — provider comes from a URL path segment, so it's validated against
// this map rather than interpolated into SQL directly.
var aiProviderKeyColumns = map[string]string{
	"gemini": "gemini_api_key",
	"groq":   "groq_api_key",
	"claude": "claude_api_key",
}

// UpdateAIProviderKey saves a studio's API key for one AI provider. A small
// dedicated setter (like SetCommunicationStyleProfile/SetTrialPageLayout
// above) rather than another parameter on the already-large Update, since
// the AI Assistant settings page saves one provider's key at a time.
func (r *Repo) UpdateAIProviderKey(ctx context.Context, studioID uuid.UUID, provider, key string) error {
	col, ok := aiProviderKeyColumns[provider]
	if !ok {
		return fmt.Errorf("unsupported ai provider %q", provider)
	}
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE studios SET %s = $2, updated_at = now() WHERE id = $1`, col), studioID, key)
	if err != nil {
		return fmt.Errorf("update ai provider key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.evict(studioID)
	return nil
}

func scanStudio(row pgx.Row, cipher *secrets.Cipher) (*Studio, error) {
	var s Studio
	if err := row.Scan(&s.ID, &s.Slug, &s.Name, &s.BrandColor, &s.LogoURL, &s.ContactEmail, &s.ContactPhone,
		&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.GeminiAPIKey, &s.GroqAPIKey, &s.ClaudeAPIKey, &s.MetaAppID, &s.MetaAppSecret,
		&s.GoogleClientID, &s.GoogleClientSecret, &s.GoogleDeveloperToken,
		&s.StripeAccountID, &s.StripeSecretKey, &s.StripePublishableKey, &s.StripeWebhookSecret, &s.SubscriptionTier, &s.SocialPlannerEnabled, &s.KnowledgeBase, &s.KnowledgeBaseFiles,
		&s.GreetingMessage, &s.TrialAmountSGD, &s.ManagedBy1Hero, &s.BookingHeroImageURL, &s.BookingHeroVideoURL,
		&s.TrialConfirmationMessage, &s.MembershipConfirmationMessage,
		&s.TrialGlofoxMembershipID, &s.TrialGlofoxPlanCode, &s.MembershipGlofoxMembershipID, &s.MembershipGlofoxPlanCode,
		&s.CommunicationStyleProfile, &s.StyleProfileUpdatedAt, &s.StyleRefreshIntervalMinutes, &s.ProgramStartDate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan studio: %w", err)
	}

	if s.StripeSecretKey != "" && cipher != nil {
		dec, err := cipher.Decrypt(s.StripeSecretKey)
		if err == nil {
			s.StripeSecretKey = dec
		}
	}
	if s.StripeWebhookSecret != "" && cipher != nil {
		dec, err := cipher.Decrypt(s.StripeWebhookSecret)
		if err == nil {
			s.StripeWebhookSecret = dec
		}
	}

	return &s, nil
}

func (r *Repo) UpdatePayments(ctx context.Context, id uuid.UUID, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier string) error {
	var err error
	if stripeSecretKey != "" && r.cipher != nil {
		stripeSecretKey, err = r.cipher.Encrypt(stripeSecretKey)
		if err != nil {
			return fmt.Errorf("encrypt stripe secret: %w", err)
		}
	}
	if stripeWebhookSecret != "" && r.cipher != nil {
		stripeWebhookSecret, err = r.cipher.Encrypt(stripeWebhookSecret)
		if err != nil {
			return fmt.Errorf("encrypt stripe webhook secret: %w", err)
		}
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET stripe_account_id = $2, stripe_secret_key = $3, stripe_publishable_key = $4, stripe_webhook_secret = $5, subscription_tier = $6, updated_at = now()
		WHERE id = $1`,
		id, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier)
	if err != nil {
		return fmt.Errorf("update payments: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Cache eviction
	r.evict(id)
	return nil
}

func (r *Repo) evict(id uuid.UUID) {
	idKey := "studio:id:" + id.String()
	if val, ok := r.cache.Get(idKey); ok {
		if s, ok := val.(*Studio); ok {
			r.cache.Evict("studio:slug:" + s.Slug)
		}
	}
	r.cache.Evict(idKey)
}

func (r *Repo) UpdatePlatformSetting(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_settings (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, key, value)
	if err == nil {
		r.cache.Evict("pset:" + key)
	}
	return err
}

func (r *Repo) GetPlatformSetting(ctx context.Context, key string) (string, error) {
	cacheKey := "pset:" + key
	if v, ok := r.cache.Get(cacheKey); ok {
		if s, ok := v.(string); ok {
			return s, nil
		}
	}
	var value string
	err := r.pool.QueryRow(ctx, "SELECT value FROM platform_settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.cache.Set(cacheKey, "", 10*time.Minute)
			return "", nil
		}
		return "", err
	}
	r.cache.Set(cacheKey, value, 10*time.Minute)
	return value, nil
}

// ── RAG / Knowledge Chunks ──────────────────────────────────

type ChunkData struct {
	SourceType string
	SourceName string
	Platform   string // "all","whatsapp","instagram","facebook","sms"
	Index      int
	Content    string
	Embedding  []float32
}

// SaveKnowledgeChunks atomically replaces all chunks for a studio.
func (r *Repo) SaveKnowledgeChunks(ctx context.Context, studioID uuid.UUID, chunks []ChunkData) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, "DELETE FROM studio_knowledge_chunks WHERE studio_id = $1", studioID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	for _, c := range chunks {
		embStr := FormatVectorAsString(c.Embedding)
		platform := c.Platform
		if platform == "" {
			platform = "all"
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO studio_knowledge_chunks (studio_id, source_type, source_name, platform, chunk_index, content, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7::vector)
		`, studioID, c.SourceType, c.SourceName, platform, c.Index, c.Content, embStr)
		if err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// SetKnowledgeSyncStatus records the background embedding sync's progress —
// "syncing" when asyncSyncKnowledgeChunks starts, "complete"/"error" when it
// finishes — so the admin UI can poll and tell the admin when it's actually
// safe to test a just-uploaded document, instead of the misleading
// impression that saving the file means it's already searchable.
func (r *Repo) SetKnowledgeSyncStatus(ctx context.Context, studioID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET knowledge_sync_status = $2, knowledge_sync_updated_at = now()
		WHERE id = $1
	`, studioID, status)
	return err
}

// GetKnowledgeSyncStatus returns the current status ("idle"/"syncing"/
// "complete"/"error") and when it last changed.
func (r *Repo) GetKnowledgeSyncStatus(ctx context.Context, studioID uuid.UUID) (string, *time.Time, error) {
	var status string
	var updatedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT knowledge_sync_status, knowledge_sync_updated_at FROM studios WHERE id = $1
	`, studioID).Scan(&status, &updatedAt)
	return status, updatedAt, err
}

// KnowledgeChunkMatch is one retrieved chunk plus the uploaded source it came
// from. SourceName round-trips to the Test Chat UI so an admin can see which
// document actually backed a given answer — see messaging.TestChat.
type KnowledgeChunkMatch struct {
	Content    string
	SourceName string
}

// SearchKnowledgeChunks performs hybrid retrieval: vector similarity + BM25 full-text search,
// fused with Reciprocal Rank Fusion (RRF). Returns top-K deduplicated chunks.
// Falls back to pure vector search if the FTS column is not yet available.
// platform filters to chunks tagged for that channel plus "all" chunks; pass "" or "all" to skip filtering.
func (r *Repo) SearchKnowledgeChunks(ctx context.Context, studioID uuid.UUID, queryEmbedding []float32, platform string, limit int) ([]KnowledgeChunkMatch, error) {
	embStr := FormatVectorAsString(queryEmbedding)
	candidateN := limit * 5
	if platform == "" {
		platform = "all"
	}

	rows, err := r.pool.Query(ctx, `
		WITH vector_ranked AS (
			SELECT content, source_name, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			ORDER BY embedding <=> $2::vector
			LIMIT $5
		),
		fts_ranked AS (
			SELECT content, source_name, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			  AND content_tsv @@ plainto_tsquery('english', $4)
			ORDER BY ts_rank(content_tsv, plainto_tsquery('english', $4)) DESC
			LIMIT $5
		),
		fused AS (
			SELECT content,
				COALESCE(v.source_name, f.source_name) AS source_name,
				COALESCE(v.rrf, 0) + COALESCE(f.rrf, 0) AS score
			FROM (
				SELECT content, source_name, 1.0 / (60 + rank) AS rrf FROM vector_ranked
			) v
			FULL OUTER JOIN (
				SELECT content, source_name, 1.0 / (60 + rank) AS rrf FROM fts_ranked
			) f USING (content)
		)
		-- content ASC tiebreaker — see SearchKnowledgeChunksHybrid's comment.
		SELECT content, source_name FROM fused
		ORDER BY score DESC, content ASC
		LIMIT $6
	`, studioID, embStr, platform, "", candidateN, limit)

	if err != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT content, source_name
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			ORDER BY embedding <=> $2::vector
			LIMIT $4
		`, studioID, embStr, platform, limit)
		if err != nil {
			return nil, fmt.Errorf("search chunks: %w", err)
		}
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var results []KnowledgeChunkMatch
	for rows.Next() {
		var content, sourceName string
		if err := rows.Scan(&content, &sourceName); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		if !seen[content] {
			seen[content] = true
			results = append(results, KnowledgeChunkMatch{Content: content, SourceName: sourceName})
		}
	}
	return results, rows.Err()
}

// SearchKnowledgeChunksHybrid is the same as SearchKnowledgeChunks but accepts
// an explicit query string for BM25 (used when the caller has a plain-text query).
// platform filters chunks for the conversation's channel; "all" returns everything.
func (r *Repo) SearchKnowledgeChunksHybrid(ctx context.Context, studioID uuid.UUID, queryEmbedding []float32, queryText string, platform string, limit int) ([]KnowledgeChunkMatch, error) {
	embStr := FormatVectorAsString(queryEmbedding)
	candidateN := limit * 5
	if platform == "" {
		platform = "all"
	}

	rows, err := r.pool.Query(ctx, `
		WITH vector_ranked AS (
			SELECT content, source_name, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			ORDER BY embedding <=> $2::vector
			LIMIT $5
		),
		fts_ranked AS (
			SELECT content, source_name, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			  AND content_tsv @@ plainto_tsquery('english', $4)
			ORDER BY ts_rank(content_tsv, plainto_tsquery('english', $4)) DESC
			LIMIT $5
		),
		fused AS (
			SELECT content,
				COALESCE(v.source_name, f.source_name) AS source_name,
				COALESCE(v.rrf, 0) + COALESCE(f.rrf, 0) AS score
			FROM (
				SELECT content, source_name, 1.0 / (60 + rank) AS rrf FROM vector_ranked
			) v
			FULL OUTER JOIN (
				SELECT content, source_name, 1.0 / (60 + rank) AS rrf FROM fts_ranked
			) f USING (content)
		)
		-- content ASC breaks exact score ties deterministically: without it,
		-- Postgres doesn't guarantee a stable order among tied rows, so the
		-- same query could hand RerankChunks a different candidate set
		-- (and drop a relevant chunk) from one request to the next.
		SELECT content, source_name FROM fused
		ORDER BY score DESC, content ASC
		LIMIT $6
	`, studioID, embStr, platform, queryText, candidateN, limit)

	if err != nil {
		return r.SearchKnowledgeChunks(ctx, studioID, queryEmbedding, platform, limit)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var results []KnowledgeChunkMatch
	for rows.Next() {
		var content, sourceName string
		if err := rows.Scan(&content, &sourceName); err != nil {
			return nil, fmt.Errorf("scan chunk hybrid: %w", err)
		}
		if !seen[content] {
			seen[content] = true
			results = append(results, KnowledgeChunkMatch{Content: content, SourceName: sourceName})
		}
	}
	return results, rows.Err()
}
