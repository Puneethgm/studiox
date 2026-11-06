package studios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// encryptOrPlain encrypts an AI provider API key before it's written to the
// studios table — same AES-256-GCM cipher already used for
// stripe_secret_key/stripe_webhook_secret. Empty values and a nil cipher
// (e.g. TOKEN_ENCRYPTION_KEY unset in a dev env) pass through unchanged
// rather than failing the whole write.
func (r *Repo) encryptOrPlain(key string) (string, error) {
	if key == "" || r.cipher == nil {
		return key, nil
	}
	return r.cipher.Encrypt(key)
}

// decryptOrPlain reverses encryptOrPlain. Falls back to returning the raw
// stored value on a decrypt error rather than erroring the whole read — this
// is what makes the switch to encrypted storage non-destructive: a row
// written before this field was encrypted is still valid ciphertext-shaped
// garbage to Decrypt (fails the GCM tag check) and the caller just gets the
// plaintext back unchanged, same as always, until it's next saved and
// becomes genuinely encrypted. Same pattern already used for
// stripe_secret_key/stripe_webhook_secret below.
func decryptOrPlain(cipher *secrets.Cipher, value string) string {
	if value == "" || cipher == nil {
		return value
	}
	dec, err := cipher.Decrypt(value)
	if err != nil {
		return value
	}
	return dec
}

func (r *Repo) Create(ctx context.Context, tx pgx.Tx, s *Studio, actorID *uuid.UUID) error {
	var err error
	encWebhookSecret := s.StripeWebhookSecret
	if encWebhookSecret != "" && r.cipher != nil {
		encWebhookSecret, err = r.cipher.Encrypt(encWebhookSecret)
		if err != nil {
			return fmt.Errorf("encrypt stripe webhook secret: %w", err)
		}
	}
	kbFilesJSON, _ := json.Marshal(s.KnowledgeBaseFiles)
	currency := s.Currency
	if currency == "" {
		currency = "SGD"
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO studios (slug, name, brand_color, logo_url, contact_email, contact_phone, active, meta_app_id, meta_app_secret, google_client_id, google_client_secret, google_developer_token, stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files, trial_amount_sgd, managed_by_1hero, currency, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$24)
		RETURNING id, created_at, updated_at
	`, s.Slug, s.Name, s.BrandColor, s.LogoURL, s.ContactEmail, s.ContactPhone, s.Active, s.MetaAppID, s.MetaAppSecret, s.GoogleClientID, s.GoogleClientSecret, s.GoogleDeveloperToken, s.StripeAccountID, s.StripeSecretKey, s.StripePublishableKey, encWebhookSecret, s.SubscriptionTier, s.SocialPlannerEnabled, s.KnowledgeBase, string(kbFilesJSON), s.TrialAmountSGD, s.ManagedBy1Hero, currency, actorID)
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
		       s.active, s.created_at, s.updated_at, s.availability_slots, s.availability_timezone, s.meta_app_id, s.meta_app_secret,
		       s.google_client_id, s.google_client_secret, s.google_developer_token,
		       s.stripe_account_id, s.stripe_secret_key, s.stripe_publishable_key, s.stripe_webhook_secret, s.subscription_tier, s.social_planner_enabled, s.knowledge_base, s.knowledge_base_files,
		       s.trial_amount_sgd, s.managed_by_1hero, s.ai_access_enabled,
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
			&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.MetaAppID, &s.MetaAppSecret,
			&s.GoogleClientID, &s.GoogleClientSecret, &s.GoogleDeveloperToken,
			&s.StripeAccountID, &s.StripeSecretKey, &s.StripePublishableKey, &s.StripeWebhookSecret, &s.SubscriptionTier, &s.SocialPlannerEnabled, &s.KnowledgeBase, &s.KnowledgeBaseFiles,
			&s.TrialAmountSGD, &s.ManagedBy1Hero, &s.AIAccessEnabled, &s.CampaignCount, &s.LeadCount); err != nil {
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
		       availability_slots, availability_timezone, mistral_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url,
		       trial_confirmation_message, membership_confirmation_message,
		       trial_glofox_membership_id, trial_glofox_plan_code, membership_glofox_membership_id, membership_glofox_plan_code,
		       communication_style_profile, style_profile_updated_at, style_refresh_interval_minutes, program_start_date, currency, ai_access_enabled, mistral_api_key_fingerprint, reply_tone
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
		       availability_slots, availability_timezone, mistral_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url,
		       trial_confirmation_message, membership_confirmation_message,
		       trial_glofox_membership_id, trial_glofox_plan_code, membership_glofox_membership_id, membership_glofox_plan_code,
		       communication_style_profile, style_profile_updated_at, style_refresh_interval_minutes, program_start_date, currency, ai_access_enabled, mistral_api_key_fingerprint, reply_tone
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
func (r *Repo) Update(ctx context.Context, id uuid.UUID, name, brandColor, logoURL, contactEmail, contactPhone string, active bool, managedBy1Hero bool, availabilitySlots []AvailabilitySlot, availabilityTimezone string, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken string, socialPlannerEnabled bool, knowledgeBase string, knowledgeBaseFiles []KnowledgeBaseFile, greetingMessage string, trialAmountSGD int, bookingHeroImageURL, bookingHeroVideoURL string, trialConfirmationMessage, membershipConfirmationMessage string, trialGlofoxMembershipID, trialGlofoxPlanCode, membershipGlofoxMembershipID, membershipGlofoxPlanCode string, currency string, actorID *uuid.UUID) error {
	slotsJSON, _ := json.Marshal(availabilitySlots)
	filesJSON, _ := json.Marshal(knowledgeBaseFiles)
	if currency == "" {
		currency = "SGD"
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET name = $2, brand_color = $3, logo_url = $4, contact_email = $5, contact_phone = $6, active = $7,
		    availability_slots = $8, availability_timezone = $9,
		    meta_app_id = $10, meta_app_secret = $11, google_client_id = $12, google_client_secret = $13, google_developer_token = $14,
		    social_planner_enabled = $15, knowledge_base = $16, knowledge_base_files = $17,
		    greeting_message = $18, trial_amount_sgd = $19, managed_by_1hero = $20, booking_hero_image_url = $21, booking_hero_video_url = $22,
		    trial_confirmation_message = $23, membership_confirmation_message = $24,
		    trial_glofox_membership_id = $25, trial_glofox_plan_code = $26, membership_glofox_membership_id = $27, membership_glofox_plan_code = $28,
		    currency = $29,
		    updated_by = $30, updated_at = now()
		WHERE id = $1`,
		id, name, brandColor, logoURL, contactEmail, contactPhone, active, string(slotsJSON), availabilityTimezone, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken, socialPlannerEnabled, knowledgeBase, string(filesJSON), greetingMessage, trialAmountSGD, managedBy1Hero, bookingHeroImageURL, bookingHeroVideoURL, trialConfirmationMessage, membershipConfirmationMessage, trialGlofoxMembershipID, trialGlofoxPlanCode, membershipGlofoxMembershipID, membershipGlofoxPlanCode, currency, actorID)
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

// GetStaffActiveWindowMinutes returns how recently a staff-authored message
// in a conversation counts as "staff is actively handling this" for the AI
// worker's staff-active reply gate (messaging.checkReplyGates) — the AI
// stays silent for this long after any staff message in that conversation.
// Previously a hardcoded 10-minute constant; now per-studio.
func (r *Repo) GetStaffActiveWindowMinutes(ctx context.Context, studioID uuid.UUID) (int, error) {
	var minutes int
	err := r.pool.QueryRow(ctx, `
		SELECT staff_active_window_minutes FROM studios WHERE id = $1
	`, studioID).Scan(&minutes)
	if err != nil {
		return 0, err
	}
	return minutes, nil
}

func (r *Repo) SetStaffActiveWindowMinutes(ctx context.Context, studioID uuid.UUID, minutes int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE studios SET staff_active_window_minutes = $2, updated_at = now() WHERE id = $1
	`, studioID, minutes)
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

// ValidReplyTones is the allowed set for SetReplyTone and the HTTP
// handler's validation — mirrors the DB CHECK constraint added by
// migrations/20261102010001_reply_tone.sql (defense in depth: reject
// before the query, not just rely on the constraint erroring).
var ValidReplyTones = map[string]bool{
	"professional": true,
	"casual":       true,
	"friendly":     true,
}

// SetReplyTone persists a studio's chosen AI-reply tone. Returns
// ErrValidation if tone isn't one of ValidReplyTones.
func (r *Repo) SetReplyTone(ctx context.Context, studioID uuid.UUID, tone string) error {
	if !ValidReplyTones[tone] {
		return ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE studios SET reply_tone = $2, updated_at = now() WHERE id = $1
	`, studioID, tone)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.evict(studioID)
	return nil
}

// aiProviderKeyColumns whitelists which column UpdateAIProviderKey may write
// to — provider comes from a URL path segment, so it's validated against
// this map rather than interpolated into SQL directly. Only mistral (OCR,
// internal/studios/knowledge_base_ocr.go) stays per-studio BYOK — groq/
// gemini/claude moved to a single centralized, superadmin-managed key (see
// GetPlatformAIProviderKey/SetPlatformAIProviderKey) so AI reply quality is
// consistent across studios instead of depending on each studio bringing
// its own key.
var aiProviderKeyColumns = map[string]string{
	"mistral": "mistral_api_key",
}

// EncryptLegacyAIProviderKeys is a one-time (idempotent, safe to re-run)
// backfill for rows written before UpdateAIProviderKey/Update/Create started
// encrypting this column — those rows are still sitting as plaintext in the
// DB and won't self-heal until that studio happens to re-save a key. Only
// mistral remains a per-studio key; groq/gemini/claude moved to centralized
// platform keys (see aiProviderKeyColumns' doc comment).
// For each studio: if the stored mistral key fails to decrypt (i.e. it's
// plaintext, not already ciphertext from this cipher) and isn't empty,
// re-save it through the now-encrypting write path. Returns how many rows
// were actually re-encrypted.
func (r *Repo) EncryptLegacyAIProviderKeys(ctx context.Context) (int, error) {
	if r.cipher == nil {
		return 0, errors.New("TOKEN_ENCRYPTION_KEY not configured — cannot encrypt")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, mistral_api_key FROM studios
	`)
	if err != nil {
		return 0, fmt.Errorf("list studios for key backfill: %w", err)
	}
	type row struct {
		id      uuid.UUID
		mistral string
	}
	var toFix []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.id, &rr.mistral); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan studio for key backfill: %w", err)
		}
		toFix = append(toFix, rr)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// isPlaintext reports whether value is non-empty and NOT already valid
	// ciphertext under this cipher (Decrypt succeeds only for real
	// ciphertext — a GCM auth-tag check, not just base64-decodability).
	isPlaintext := func(value string) bool {
		if value == "" {
			return false
		}
		_, err := r.cipher.Decrypt(value)
		return err != nil
	}

	fixed := 0
	for _, rr := range toFix {
		if !isPlaintext(rr.mistral) {
			continue
		}
		if err := r.UpdateAIProviderKey(ctx, rr.id, "mistral", rr.mistral, nil); err != nil {
			return fixed, fmt.Errorf("re-encrypt mistral key for studio %s: %w", rr.id, err)
		}
		fixed++
	}
	return fixed, nil
}

// UpdateAIProviderKey saves a studio's API key for the one remaining
// per-studio AI provider (mistral/OCR). A small dedicated setter (like
// SetCommunicationStyleProfile/SetTrialPageLayout above) rather than another
// parameter on the already-large Update.
func (r *Repo) UpdateAIProviderKey(ctx context.Context, studioID uuid.UUID, provider, key string, actorID *uuid.UUID) error {
	col, ok := aiProviderKeyColumns[provider]
	if !ok {
		return fmt.Errorf("unsupported ai provider %q", provider)
	}
	encKey, err := r.encryptOrPlain(key)
	if err != nil {
		return fmt.Errorf("encrypt ai provider key: %w", err)
	}
	// mistral is the only per-studio BYOK key left (see aiProviderKeyColumns'
	// doc comment) and the only one with a fingerprint column today — a
	// non-reversible stand-in for keySuffix()'s removed last-4-chars reveal
	// (platform-security-hardening task 0.2).
	if provider == "mistral" {
		tag, err := r.pool.Exec(ctx, `
			UPDATE studios SET mistral_api_key = $2, mistral_api_key_fingerprint = $3, updated_by = $4, updated_at = now() WHERE id = $1
		`, studioID, encKey, fingerprintSecret(key), actorID)
		if err != nil {
			return fmt.Errorf("update ai provider key: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		r.evict(studioID)
		return nil
	}
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE studios SET %s = $2, updated_by = $3, updated_at = now() WHERE id = $1`, col), studioID, encKey, actorID)
	if err != nil {
		return fmt.Errorf("update ai provider key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.evict(studioID)
	return nil
}

// SetAIAccessEnabled toggles the superadmin-only master switch for whether
// this studio gets AI replies at all (see Studio.AIAccessEnabled's doc
// comment). A small dedicated setter, same convention as
// UpdateAIProviderKey/SetCommunicationStyleProfile above, rather than
// another parameter on the already-large Update — and deliberately NOT
// exposed through the studio-self-service update path, since only a
// superadmin may call this.
func (r *Repo) SetAIAccessEnabled(ctx context.Context, studioID uuid.UUID, enabled bool, actorID *uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE studios SET ai_access_enabled = $2, updated_by = $3, updated_at = now() WHERE id = $1`, studioID, enabled, actorID)
	if err != nil {
		return fmt.Errorf("set ai access enabled: %w", err)
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
		&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.MistralAPIKey, &s.MetaAppID, &s.MetaAppSecret,
		&s.GoogleClientID, &s.GoogleClientSecret, &s.GoogleDeveloperToken,
		&s.StripeAccountID, &s.StripeSecretKey, &s.StripePublishableKey, &s.StripeWebhookSecret, &s.SubscriptionTier, &s.SocialPlannerEnabled, &s.KnowledgeBase, &s.KnowledgeBaseFiles,
		&s.GreetingMessage, &s.TrialAmountSGD, &s.ManagedBy1Hero, &s.BookingHeroImageURL, &s.BookingHeroVideoURL,
		&s.TrialConfirmationMessage, &s.MembershipConfirmationMessage,
		&s.TrialGlofoxMembershipID, &s.TrialGlofoxPlanCode, &s.MembershipGlofoxMembershipID, &s.MembershipGlofoxPlanCode,
		&s.CommunicationStyleProfile, &s.StyleProfileUpdatedAt, &s.StyleRefreshIntervalMinutes, &s.ProgramStartDate, &s.Currency, &s.AIAccessEnabled, &s.MistralAPIKeyFingerprint, &s.ReplyTone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan studio: %w", err)
	}

	s.MistralAPIKey = decryptOrPlain(cipher, s.MistralAPIKey)

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

func (r *Repo) UpdatePayments(ctx context.Context, id uuid.UUID, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier string, actorID *uuid.UUID) error {
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
		SET stripe_account_id = $2, stripe_secret_key = $3, stripe_publishable_key = $4, stripe_webhook_secret = $5, subscription_tier = $6, updated_by = $7, updated_at = now()
		WHERE id = $1`,
		id, stripeAccountId, stripeSecretKey, stripePublishableKey, stripeWebhookSecret, subscriptionTier, actorID)
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

func (r *Repo) UpdatePlatformSetting(ctx context.Context, key, value string, actorID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()
	`, key, value, actorID)
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

// platformAIProviderKeyNames whitelists which platform_settings row each
// centralized AI provider key lives at — provider comes from a URL path
// segment in the superadmin AI Provider Settings page, so it's validated
// against this map rather than interpolated into a key name directly.
var platformAIProviderKeyNames = map[string]string{
	"groq":   "groq_api_key",
	"gemini": "gemini_api_key",
	"claude": "claude_api_key",
	"openai": "openai_api_key",
}

// AI provider key audit actions recorded in platform_ai_key_audit.
const (
	AIKeyAuditSaved    = "saved"
	AIKeyAuditRotated  = "rotated"
	AIKeyAuditDisabled = "disabled"
	AIKeyAuditEnabled  = "enabled"
)

// PlatformAIKeyStatus is a provider key's lifecycle status as stored in
// platform_ai_keys.status.
const (
	AIKeyStatusActive   = "active"
	AIKeyStatusDisabled = "disabled"
	AIKeyStatusFailing  = "failing"
)

// GetPlatformAIProviderKey returns the centrally-configured, decrypted key
// for provider (groq/gemini/claude/openai) from the envelope-encrypted
// platform_ai_keys store: the key is decrypted with its own per-row data
// key, which is itself decrypted with the app's master cipher
// (TOKEN_ENCRYPTION_KEY) — see
// openspec/changes/llm-gateway-and-key-management/design.md Decision 4. A
// missing or disabled key returns ("", nil), not an error, so callers
// treat it the same as "provider not configured" — same contract as
// before this table existed.
func (r *Repo) GetPlatformAIProviderKey(ctx context.Context, provider string) (string, error) {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return "", fmt.Errorf("unsupported ai provider %q", provider)
	}
	var keyCiphertext, dataKeyCiphertext, status string
	err := r.pool.QueryRow(ctx, `
		SELECT key_ciphertext, data_key_ciphertext, status FROM platform_ai_keys WHERE provider = $1
	`, provider).Scan(&keyCiphertext, &dataKeyCiphertext, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get platform ai provider key: %w", err)
	}
	if status == AIKeyStatusDisabled {
		return "", nil
	}
	if r.cipher == nil {
		return "", errors.New("TOKEN_ENCRYPTION_KEY not configured — cannot decrypt platform AI provider keys")
	}
	dataKey, err := r.cipher.Decrypt(dataKeyCiphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt data key: %w", err)
	}
	key, err := secrets.DecryptWithDataKey(dataKey, keyCiphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt provider key: %w", err)
	}
	return key, nil
}

// GetPlatformAIProviderKeyFingerprint reports whether provider has a
// usable (non-disabled) key configured and, if so, its fingerprint —
// without ever decrypting the key itself. Used by the AI Provider
// Settings list, which previously decrypted the real key just to show
// its last 4 characters (platform-security-hardening task 0.2).
func (r *Repo) GetPlatformAIProviderKeyFingerprint(ctx context.Context, provider string) (hasKey bool, fingerprint string, err error) {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return false, "", fmt.Errorf("unsupported ai provider %q", provider)
	}
	var fp, status string
	err = r.pool.QueryRow(ctx, `
		SELECT key_fingerprint, status FROM platform_ai_keys WHERE provider = $1
	`, provider).Scan(&fp, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("get platform ai provider key fingerprint: %w", err)
	}
	if status == AIKeyStatusDisabled {
		return false, "", nil
	}
	return true, fp, nil
}

// SetPlatformAIProviderKey envelope-encrypts and saves the centrally-
// configured key for provider: a freshly generated data key encrypts the
// key itself, and the master cipher encrypts that data key (which is the
// only thing ever persisted for it — the raw data key never touches the
// database). The key is (re)activated (status=active) and the save is
// audited. See GetPlatformAIProviderKey and design.md Decision 4.
func (r *Repo) SetPlatformAIProviderKey(ctx context.Context, provider, key string, actorID *uuid.UUID) error {
	return r.setPlatformAIProviderKey(ctx, provider, key, actorID, AIKeyAuditSaved)
}

// RotatePlatformAIProviderKey replaces provider's key with a freshly
// generated envelope (new data key, new ciphertexts) the same way
// SetPlatformAIProviderKey does, but records the audit action as
// "rotated" rather than "saved" — the superadmin-facing lifecycle action
// distinct from a first-time save (task 2.6).
func (r *Repo) RotatePlatformAIProviderKey(ctx context.Context, provider, key string, actorID *uuid.UUID) error {
	return r.setPlatformAIProviderKey(ctx, provider, key, actorID, AIKeyAuditRotated)
}

func (r *Repo) setPlatformAIProviderKey(ctx context.Context, provider, key string, actorID *uuid.UUID, auditAction string) error {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return fmt.Errorf("unsupported ai provider %q", provider)
	}
	if r.cipher == nil {
		return errors.New("TOKEN_ENCRYPTION_KEY not configured — cannot store platform AI provider keys")
	}
	dataKey, err := secrets.GenerateDataKey()
	if err != nil {
		return fmt.Errorf("generate data key: %w", err)
	}
	keyCiphertext, err := secrets.EncryptWithDataKey(dataKey, key)
	if err != nil {
		return fmt.Errorf("encrypt provider key: %w", err)
	}
	dataKeyCiphertext, err := r.cipher.Encrypt(dataKey)
	if err != nil {
		return fmt.Errorf("encrypt data key: %w", err)
	}

	var actorArg any
	if actorID != nil {
		actorArg = *actorID
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO platform_ai_keys (provider, key_ciphertext, data_key_ciphertext, key_fingerprint, status, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, now(), $6)
		ON CONFLICT (provider) DO UPDATE SET
			key_ciphertext = EXCLUDED.key_ciphertext,
			data_key_ciphertext = EXCLUDED.data_key_ciphertext,
			key_fingerprint = EXCLUDED.key_fingerprint,
			status = EXCLUDED.status,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by
	`, provider, keyCiphertext, dataKeyCiphertext, fingerprintSecret(key), AIKeyStatusActive, actorArg)
	if err != nil {
		return fmt.Errorf("upsert platform ai key: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO platform_ai_key_audit (provider, action, actor_id) VALUES ($1, $2, $3)
	`, provider, auditAction, actorArg); err != nil {
		return fmt.Errorf("audit platform ai key %s: %w", auditAction, err)
	}
	return tx.Commit(ctx)
}

// GetPlatformAIProviderKeyStatus returns provider's key lifecycle status
// ("" if no key has ever been saved), without decrypting anything — used
// by the AI Provider Settings page to show active/disabled/failing
// without paying a decrypt on every page load.
func (r *Repo) GetPlatformAIProviderKeyStatus(ctx context.Context, provider string) (string, error) {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return "", fmt.Errorf("unsupported ai provider %q", provider)
	}
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM platform_ai_keys WHERE provider = $1`, provider).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get platform ai key status: %w", err)
	}
	return status, nil
}

// MarkPlatformAIProviderKeyFailing flips provider's key status to
// "failing" — a system-detected state (repeated authentication-type
// errors from the provider, not a user action), distinct from the
// user-driven active/disabled toggle. A no-op if there's no saved key for
// the provider (nothing to flag) or it's already failing (avoids a flood
// of duplicate audit rows while the key stays broken).
func (r *Repo) MarkPlatformAIProviderKeyFailing(ctx context.Context, provider string) error {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return fmt.Errorf("unsupported ai provider %q", provider)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE platform_ai_keys SET status = $2, updated_at = now()
		WHERE provider = $1 AND status != $2
	`, provider, AIKeyStatusFailing)
	if err != nil {
		return fmt.Errorf("mark platform ai key failing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO platform_ai_key_audit (provider, action) VALUES ($1, 'flagged_failing')
	`, provider); err != nil {
		return fmt.Errorf("audit platform ai key failing: %w", err)
	}
	return tx.Commit(ctx)
}

// SetPlatformAIProviderKeyStatus disables or re-enables provider's key
// without touching its ciphertext — a disabled key makes
// GetPlatformAIProviderKey return "" (same as "not configured") until
// re-enabled, and the change is audited. status must be
// AIKeyStatusActive or AIKeyStatusDisabled.
func (r *Repo) SetPlatformAIProviderKeyStatus(ctx context.Context, provider, status string, actorID *uuid.UUID) error {
	if _, ok := platformAIProviderKeyNames[provider]; !ok {
		return fmt.Errorf("unsupported ai provider %q", provider)
	}
	if status != AIKeyStatusActive && status != AIKeyStatusDisabled {
		return fmt.Errorf("invalid status %q", status)
	}
	var actorArg any
	if actorID != nil {
		actorArg = *actorID
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE platform_ai_keys SET status = $2, updated_at = now(), updated_by = $3 WHERE provider = $1
	`, provider, status, actorArg)
	if err != nil {
		return fmt.Errorf("update platform ai key status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no key saved for provider %q", provider)
	}
	action := AIKeyAuditEnabled
	if status == AIKeyStatusDisabled {
		action = AIKeyAuditDisabled
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO platform_ai_key_audit (provider, action, actor_id) VALUES ($1, $2, $3)
	`, provider, action, actorArg); err != nil {
		return fmt.Errorf("audit platform ai key status change: %w", err)
	}
	return tx.Commit(ctx)
}

// MigrateLegacyAIProviderKeys moves any provider key still sitting in the
// old platform_settings storage (groq_api_key/gemini_api_key/
// claude_api_key/openai_api_key, single-static-key AES, pre-envelope
// encryption) into the new envelope-encrypted platform_ai_keys table, via
// the same SetPlatformAIProviderKey path every other save goes through.
// Idempotent and safe to re-run: a provider already present in
// platform_ai_keys is left untouched (its row always wins over the legacy
// value), and a provider with no legacy value is skipped. Returns the list
// of providers actually migrated this run. See task 2.5 — one-off
// operator-run step, invoked via cmd/backfill-ai-keys.
func (r *Repo) MigrateLegacyAIProviderKeys(ctx context.Context) ([]string, error) {
	var migrated []string
	for provider, settingKey := range platformAIProviderKeyNames {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_ai_keys WHERE provider = $1)`, provider).Scan(&exists); err != nil {
			return migrated, fmt.Errorf("check existing platform_ai_keys row for %s: %w", provider, err)
		}
		if exists {
			continue
		}
		legacyValue, err := r.GetPlatformSetting(ctx, settingKey)
		if err != nil {
			return migrated, fmt.Errorf("read legacy setting %s: %w", settingKey, err)
		}
		if legacyValue == "" {
			continue
		}
		legacyKey := decryptOrPlain(r.cipher, legacyValue)
		if legacyKey == "" {
			continue
		}
		if err := r.SetPlatformAIProviderKey(ctx, provider, legacyKey, nil); err != nil {
			return migrated, fmt.Errorf("migrate %s key: %w", provider, err)
		}
		migrated = append(migrated, provider)
	}
	return migrated, nil
}

// AIAlertThresholds are the superadmin-editable trigger points for the two
// threshold-based AI alerts (provider failure-rate spike, daily spend over
// budget) — the two alert conditions that aren't fired on a single event
// (unlike "all providers failed" or "a key started failing"), so they need
// a periodic check against a configured bar. Stored as plain
// platform_settings rows (same pattern as platformAIProviderKeyNames'
// settings, just unencrypted — these aren't secrets), not a dedicated
// table, since it's three scalar values with no lifecycle of their own.
// Defaults are conservative starting points the design doc flagged as
// "not specified in the source doc" — a superadmin should tune them once
// real spend/failure-rate data is visible on the AI Providers page.
type AIAlertThresholds struct {
	FailureRatePct       float64 `json:"failureRatePct"`
	FailureRateWindowMin int     `json:"failureRateWindowMinutes"`
	DailySpendBudgetUSD  float64 `json:"dailySpendBudgetUsd"`
}

const (
	aiAlertFailureRatePctSetting       = "ai_alert_failure_rate_pct"
	aiAlertFailureRateWindowMinSetting = "ai_alert_failure_rate_window_minutes"
	aiAlertDailySpendBudgetUSDSetting  = "ai_alert_daily_spend_budget_usd"

	defaultAIAlertFailureRatePct       = 50.0
	defaultAIAlertFailureRateWindowMin = 15
	defaultAIAlertDailySpendBudgetUSD  = 50.0
)

// GetAIAlertThresholds returns the current thresholds, falling back to
// defaults for any setting never saved (first run).
func (r *Repo) GetAIAlertThresholds(ctx context.Context) (AIAlertThresholds, error) {
	out := AIAlertThresholds{
		FailureRatePct:       defaultAIAlertFailureRatePct,
		FailureRateWindowMin: defaultAIAlertFailureRateWindowMin,
		DailySpendBudgetUSD:  defaultAIAlertDailySpendBudgetUSD,
	}
	if v, err := r.GetPlatformSetting(ctx, aiAlertFailureRatePctSetting); err != nil {
		return out, fmt.Errorf("get %s: %w", aiAlertFailureRatePctSetting, err)
	} else if v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out.FailureRatePct = f
		}
	}
	if v, err := r.GetPlatformSetting(ctx, aiAlertFailureRateWindowMinSetting); err != nil {
		return out, fmt.Errorf("get %s: %w", aiAlertFailureRateWindowMinSetting, err)
	} else if v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			out.FailureRateWindowMin = n
		}
	}
	if v, err := r.GetPlatformSetting(ctx, aiAlertDailySpendBudgetUSDSetting); err != nil {
		return out, fmt.Errorf("get %s: %w", aiAlertDailySpendBudgetUSDSetting, err)
	} else if v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out.DailySpendBudgetUSD = f
		}
	}
	return out, nil
}

// SetAIAlertThresholds validates and saves all three thresholds in one
// call — a superadmin edits them together on the settings page, so a
// partial save (two valid, one rejected) would leave a confusing
// half-applied state.
func (r *Repo) SetAIAlertThresholds(ctx context.Context, t AIAlertThresholds, actorID *uuid.UUID) error {
	if t.FailureRatePct < 0 || t.FailureRatePct > 100 {
		return fmt.Errorf("failureRatePct must be between 0 and 100")
	}
	if t.FailureRateWindowMin <= 0 {
		return fmt.Errorf("failureRateWindowMinutes must be positive")
	}
	if t.DailySpendBudgetUSD < 0 {
		return fmt.Errorf("dailySpendBudgetUsd must not be negative")
	}
	if err := r.UpdatePlatformSetting(ctx, aiAlertFailureRatePctSetting, strconv.FormatFloat(t.FailureRatePct, 'f', -1, 64), actorID); err != nil {
		return fmt.Errorf("save %s: %w", aiAlertFailureRatePctSetting, err)
	}
	if err := r.UpdatePlatformSetting(ctx, aiAlertFailureRateWindowMinSetting, strconv.Itoa(t.FailureRateWindowMin), actorID); err != nil {
		return fmt.Errorf("save %s: %w", aiAlertFailureRateWindowMinSetting, err)
	}
	if err := r.UpdatePlatformSetting(ctx, aiAlertDailySpendBudgetUSDSetting, strconv.FormatFloat(t.DailySpendBudgetUSD, 'f', -1, 64), actorID); err != nil {
		return fmt.Errorf("save %s: %w", aiAlertDailySpendBudgetUSDSetting, err)
	}
	return nil
}

// PlatformAlert is one platform_alerts row — see
// internal/messaging.Repo.RecordAlert and
// internal/integrations/llmgateway.Repo.RecordAlert, the two writers of
// this table.
type PlatformAlert struct {
	ID         uuid.UUID      `json:"id"`
	Kind       string         `json:"kind"`
	Detail     map[string]any `json:"detail"`
	StudioID   *uuid.UUID     `json:"studioId,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	ResolvedAt *time.Time     `json:"resolvedAt,omitempty"`
}

// aiAlertMinSampleSize is the minimum number of calls a provider must have
// logged within the rolling window before a failure rate is judged against
// the threshold — otherwise one failed call out of one total would always
// read as a 100% failure rate and fire immediately on near-zero traffic.
const aiAlertMinSampleSize = 5

// CheckAIAlertThresholds is the periodic (piggybacked on an existing
// worker ticker — see style_worker.go's tick, task 3.8) check for the two
// threshold-based AI alerts that aren't fired on a single event:
//   - a provider's failure rate over the configured rolling window
//     exceeds the configured percentage
//   - today's total AI spend exceeds the configured daily budget
//
// Each check is idempotent per window: it skips firing again if an
// unresolved alert of the same kind (and, for failure rate, the same
// provider) was already recorded within the current window/day.
func (r *Repo) CheckAIAlertThresholds(ctx context.Context) error {
	thresholds, err := r.GetAIAlertThresholds(ctx)
	if err != nil {
		return fmt.Errorf("get ai alert thresholds: %w", err)
	}

	window := time.Duration(thresholds.FailureRateWindowMin) * time.Minute
	for provider := range platformAIProviderKeyNames {
		var total, failed int
		if err := r.pool.QueryRow(ctx, `
			SELECT count(*), count(*) FILTER (WHERE NOT success)
			FROM llm_usage_logs
			WHERE provider = $1 AND created_at >= now() - $2::interval
		`, provider, window.String()).Scan(&total, &failed); err != nil {
			return fmt.Errorf("check failure rate for %s: %w", provider, err)
		}
		if total < aiAlertMinSampleSize {
			continue
		}
		failureRatePct := float64(failed) / float64(total) * 100
		if failureRatePct < thresholds.FailureRatePct {
			continue
		}
		var alreadyAlerted bool
		if err := r.pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM platform_alerts
				WHERE kind = 'provider_failure_rate_spike'
				AND detail->>'provider' = $1
				AND resolved_at IS NULL
				AND created_at >= now() - $2::interval
			)
		`, provider, window.String()).Scan(&alreadyAlerted); err != nil {
			return fmt.Errorf("check existing failure-rate alert for %s: %w", provider, err)
		}
		if alreadyAlerted {
			continue
		}
		detail, _ := json.Marshal(map[string]any{
			"provider":         provider,
			"failure_rate_pct": failureRatePct,
			"total_calls":      total,
			"failed_calls":     failed,
			"window_minutes":   thresholds.FailureRateWindowMin,
		})
		// string(detail), not the raw []byte from json.Marshal — see
		// internal/integrations/llmgateway.Repo.RecordAlert's comment on
		// the pgx/jsonb []byte gotcha this avoids.
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO platform_alerts (kind, detail) VALUES ('provider_failure_rate_spike', $1)
		`, string(detail)); err != nil {
			return fmt.Errorf("record failure-rate alert for %s: %w", provider, err)
		}
	}

	var todaySpendUSD float64
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(cost_usd), 0) FROM llm_usage_logs
		WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Singapore') AT TIME ZONE 'Asia/Singapore'
	`).Scan(&todaySpendUSD); err != nil {
		return fmt.Errorf("sum today's ai spend: %w", err)
	}
	if todaySpendUSD >= thresholds.DailySpendBudgetUSD {
		var alreadyAlerted bool
		if err := r.pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM platform_alerts
				WHERE kind = 'daily_spend_over_budget'
				AND resolved_at IS NULL
				AND created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Singapore') AT TIME ZONE 'Asia/Singapore'
			)
		`).Scan(&alreadyAlerted); err != nil {
			return fmt.Errorf("check existing daily-spend alert: %w", err)
		}
		if !alreadyAlerted {
			detail, _ := json.Marshal(map[string]any{
				"spend_usd":  todaySpendUSD,
				"budget_usd": thresholds.DailySpendBudgetUSD,
			})
			if _, err := r.pool.Exec(ctx, `
				INSERT INTO platform_alerts (kind, detail) VALUES ('daily_spend_over_budget', $1)
			`, string(detail)); err != nil {
				return fmt.Errorf("record daily-spend alert: %w", err)
			}
		}
	}

	return nil
}

// ListRecentPlatformAlerts returns the most recent platform_alerts rows,
// newest first, for the AI Providers settings page's alerts panel.
func (r *Repo) ListRecentPlatformAlerts(ctx context.Context, limit int) ([]PlatformAlert, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, kind, detail, studio_id, created_at, resolved_at
		FROM platform_alerts
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list platform alerts: %w", err)
	}
	defer rows.Close()

	var out []PlatformAlert
	for rows.Next() {
		var a PlatformAlert
		var detailJSON []byte
		if err := rows.Scan(&a.ID, &a.Kind, &detailJSON, &a.StudioID, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan platform alert: %w", err)
		}
		if len(detailJSON) > 0 {
			_ = json.Unmarshal(detailJSON, &a.Detail)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ── RAG / Knowledge Chunks ──────────────────────────────────

// ExistingChunkEmbedding is one already-stored chunk's content plus its
// embedding in pgvector's text form (not yet parsed to []float32 — see
// ParseVectorString) — used to detect which chunks of a re-synced
// knowledge base are byte-identical to what's already embedded, so
// asyncSyncKnowledgeChunks can reuse the embedding instead of re-calling
// the embeddings API for content that hasn't actually changed.
type ExistingChunkEmbedding struct {
	SourceType    string
	SourceName    string
	Content       string
	EmbeddingText string
}

// ListKnowledgeChunkEmbeddings returns every current chunk's (source_type,
// source_name, content) plus its embedding as pgvector text, for the reuse
// check described on ExistingChunkEmbedding.
func (r *Repo) ListKnowledgeChunkEmbeddings(ctx context.Context, studioID uuid.UUID) ([]ExistingChunkEmbedding, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT source_type, source_name, content, embedding::text
		FROM studio_knowledge_chunks
		WHERE studio_id = $1
	`, studioID)
	if err != nil {
		return nil, fmt.Errorf("list knowledge chunk embeddings: %w", err)
	}
	defer rows.Close()
	var out []ExistingChunkEmbedding
	for rows.Next() {
		var e ExistingChunkEmbedding
		if err := rows.Scan(&e.SourceType, &e.SourceName, &e.Content, &e.EmbeddingText); err != nil {
			return nil, fmt.Errorf("scan knowledge chunk embedding: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

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

// ============================================================
// knowledge_gaps — questions the AI escalated instead of guessing
// ============================================================

// CreateKnowledgeGap logs a question the AI couldn't answer confidently and had to
// hand off to a human. Repeat asks of the same (trimmed, case-insensitive) question
// while it's still open just bump times_asked instead of creating duplicate rows —
// see the partial unique index idx_knowledge_gaps_open_dedupe. Best-effort: called
// from the hot path of handling an inbound message, so a failure here must never
// block the escalation reply itself — callers should log and continue, not fail.
func (r *Repo) CreateKnowledgeGap(ctx context.Context, studioID uuid.UUID, conversationID, leadID *uuid.UUID, question string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO knowledge_gaps (studio_id, conversation_id, lead_id, question)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (studio_id, lower(btrim(question))) WHERE status = 'open'
		DO UPDATE SET times_asked = knowledge_gaps.times_asked + 1, updated_at = now(),
		              conversation_id = EXCLUDED.conversation_id, lead_id = EXCLUDED.lead_id
	`, studioID, conversationID, leadID, question)
	if err != nil {
		return fmt.Errorf("create knowledge gap: %w", err)
	}
	return nil
}

func scanKnowledgeGap(row pgx.Row) (*KnowledgeGap, error) {
	var g KnowledgeGap
	var leadName *string
	if err := row.Scan(&g.ID, &g.StudioID, &g.ConversationID, &g.LeadID, &leadName, &g.Question,
		&g.Status, &g.TimesAsked, &g.Answer, &g.CreatedAt, &g.UpdatedAt, &g.ResolvedAt); err != nil {
		return nil, err
	}
	if leadName != nil {
		g.LeadName = *leadName
	}
	return &g, nil
}

const knowledgeGapColumns = `g.id, g.studio_id, g.conversation_id, g.lead_id, l.name,
	g.question, g.status, g.times_asked, g.answer, g.created_at, g.updated_at, g.resolved_at`

// ListKnowledgeGaps returns the studio's knowledge gaps, newest first. status filters
// to "open"/"resolved"/"dismissed"; empty string returns all of them.
func (r *Repo) ListKnowledgeGaps(ctx context.Context, studioID uuid.UUID, status string) ([]KnowledgeGap, error) {
	query := `SELECT ` + knowledgeGapColumns + `
		FROM knowledge_gaps g LEFT JOIN leads l ON l.id = g.lead_id
		WHERE g.studio_id = $1`
	args := []any{studioID}
	if status != "" {
		query += ` AND g.status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY g.created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list knowledge gaps: %w", err)
	}
	defer rows.Close()

	out := make([]KnowledgeGap, 0)
	for rows.Next() {
		g, err := scanKnowledgeGap(rows)
		if err != nil {
			return nil, fmt.Errorf("scan knowledge gap: %w", err)
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

// GetKnowledgeGap fetches one gap, scoped to the studio.
func (r *Repo) GetKnowledgeGap(ctx context.Context, studioID, gapID uuid.UUID) (*KnowledgeGap, error) {
	g, err := scanKnowledgeGap(r.pool.QueryRow(ctx, `
		SELECT `+knowledgeGapColumns+`
		FROM knowledge_gaps g LEFT JOIN leads l ON l.id = g.lead_id
		WHERE g.id = $1 AND g.studio_id = $2
	`, gapID, studioID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get knowledge gap: %w", err)
	}
	return g, nil
}

// MarkKnowledgeGapResolved stores the answer and marks the gap resolved. The caller
// (Service.ResolveKnowledgeGap) is responsible for actually adding the answer into
// the studio's knowledge base — this just records that it was handled.
func (r *Repo) MarkKnowledgeGapResolved(ctx context.Context, studioID, gapID uuid.UUID, answer string, actorID *uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE knowledge_gaps
		SET status = 'resolved', answer = $3, resolved_by = $4, resolved_at = now(), updated_at = now()
		WHERE id = $1 AND studio_id = $2 AND status = 'open'
	`, gapID, studioID, answer, actorID)
	if err != nil {
		return fmt.Errorf("mark knowledge gap resolved: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkKnowledgeGapDismissed closes a gap without adding anything to the knowledge
// base — e.g. it was off-topic or a duplicate the dedupe index didn't catch.
func (r *Repo) MarkKnowledgeGapDismissed(ctx context.Context, studioID, gapID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE knowledge_gaps SET status = 'dismissed', updated_at = now()
		WHERE id = $1 AND studio_id = $2 AND status = 'open'
	`, gapID, studioID)
	if err != nil {
		return fmt.Errorf("mark knowledge gap dismissed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── AI feedback corrections ──────────────────────────────────

// CreateAIFeedbackCorrection records staff feedback on one AI reply.
func (r *Repo) CreateAIFeedbackCorrection(ctx context.Context, studioID uuid.UUID, question, originalReply, correction, source string, actorID *uuid.UUID) (*AIFeedbackCorrection, error) {
	var c AIFeedbackCorrection
	c.StudioID = studioID
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO ai_feedback_corrections (studio_id, question, original_reply, correction, source, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, question, original_reply, correction, source, created_at
	`, studioID, question, originalReply, correction, source, actorID).Scan(
		&c.ID, &c.Question, &c.OriginalReply, &c.Correction, &c.Source, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("create ai feedback correction: %w", err)
	}
	return &c, nil
}

// ListAIFeedbackCorrections returns a studio's most recent corrections,
// newest first, capped at limit. Used both by buildPrompt (recent corrections
// folded into every prompt — see the STAFF CORRECTIONS section) and by the
// admin UI listing them for review/deletion.
func (r *Repo) ListAIFeedbackCorrections(ctx context.Context, studioID uuid.UUID, limit int) ([]AIFeedbackCorrection, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, question, original_reply, correction, source, created_at
		FROM ai_feedback_corrections
		WHERE studio_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, studioID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai feedback corrections: %w", err)
	}
	defer rows.Close()
	out := make([]AIFeedbackCorrection, 0)
	for rows.Next() {
		var c AIFeedbackCorrection
		if err := rows.Scan(&c.ID, &c.StudioID, &c.Question, &c.OriginalReply, &c.Correction, &c.Source, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ai feedback correction: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteAIFeedbackCorrection removes one correction — e.g. it's stale, or it
// was itself wrong and is making the AI worse instead of better.
func (r *Repo) DeleteAIFeedbackCorrection(ctx context.Context, studioID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM ai_feedback_corrections WHERE id = $1 AND studio_id = $2
	`, id, studioID)
	if err != nil {
		return fmt.Errorf("delete ai feedback correction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
