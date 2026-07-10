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
		INSERT INTO studios (slug, name, brand_color, logo_url, contact_email, contact_phone, active, gemini_api_key, groq_api_key, meta_app_id, meta_app_secret, google_client_id, google_client_secret, google_developer_token, stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files, trial_amount_sgd, managed_by_1hero)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
		RETURNING id, created_at, updated_at
	`, s.Slug, s.Name, s.BrandColor, s.LogoURL, s.ContactEmail, s.ContactPhone, s.Active, s.GeminiAPIKey, s.GroqAPIKey, s.MetaAppID, s.MetaAppSecret, s.GoogleClientID, s.GoogleClientSecret, s.GoogleDeveloperToken, s.StripeAccountID, s.StripeSecretKey, s.StripePublishableKey, encWebhookSecret, s.SubscriptionTier, s.SocialPlannerEnabled, s.KnowledgeBase, string(kbFilesJSON), s.TrialAmountSGD, s.ManagedBy1Hero)
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
		       availability_slots, availability_timezone, gemini_api_key, groq_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url
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
		       availability_slots, availability_timezone, gemini_api_key, groq_api_key, meta_app_id, meta_app_secret,
		       google_client_id, google_client_secret, google_developer_token,
		       stripe_account_id, stripe_secret_key, stripe_publishable_key, stripe_webhook_secret, subscription_tier, social_planner_enabled, knowledge_base, knowledge_base_files,
		       greeting_message, trial_amount_sgd, managed_by_1hero, booking_hero_image_url, booking_hero_video_url
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
func (r *Repo) Update(ctx context.Context, id uuid.UUID, name, brandColor, logoURL, contactEmail, contactPhone string, active bool, managedBy1Hero bool, availabilitySlots []AvailabilitySlot, availabilityTimezone string, geminiAPIKey, groqAPIKey, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken string, socialPlannerEnabled bool, knowledgeBase string, knowledgeBaseFiles []KnowledgeBaseFile, greetingMessage string, trialAmountSGD int, bookingHeroImageURL, bookingHeroVideoURL string) error {
	slotsJSON, _ := json.Marshal(availabilitySlots)
	filesJSON, _ := json.Marshal(knowledgeBaseFiles)
	tag, err := r.pool.Exec(ctx, `
		UPDATE studios
		SET name = $2, brand_color = $3, logo_url = $4, contact_email = $5, contact_phone = $6, active = $7,
		    availability_slots = $8, availability_timezone = $9, gemini_api_key = $10, groq_api_key = $11,
		    meta_app_id = $12, meta_app_secret = $13, google_client_id = $14, google_client_secret = $15, google_developer_token = $16,
		    social_planner_enabled = $17, knowledge_base = $18, knowledge_base_files = $19,
		    greeting_message = $20, trial_amount_sgd = $21, managed_by_1hero = $22, booking_hero_image_url = $23, booking_hero_video_url = $24, updated_at = now()
		WHERE id = $1`,
		id, name, brandColor, logoURL, contactEmail, contactPhone, active, string(slotsJSON), availabilityTimezone, geminiAPIKey, groqAPIKey, metaAppID, metaAppSecret, googleClientID, googleClientSecret, googleDeveloperToken, socialPlannerEnabled, knowledgeBase, string(filesJSON), greetingMessage, trialAmountSGD, managedBy1Hero, bookingHeroImageURL, bookingHeroVideoURL)
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

func scanStudio(row pgx.Row, cipher *secrets.Cipher) (*Studio, error) {
	var s Studio
	if err := row.Scan(&s.ID, &s.Slug, &s.Name, &s.BrandColor, &s.LogoURL, &s.ContactEmail, &s.ContactPhone,
		&s.Active, &s.CreatedAt, &s.UpdatedAt, &s.AvailabilitySlots, &s.AvailabilityTimezone, &s.GeminiAPIKey, &s.GroqAPIKey, &s.MetaAppID, &s.MetaAppSecret,
		&s.GoogleClientID, &s.GoogleClientSecret, &s.GoogleDeveloperToken,
		&s.StripeAccountID, &s.StripeSecretKey, &s.StripePublishableKey, &s.StripeWebhookSecret, &s.SubscriptionTier, &s.SocialPlannerEnabled, &s.KnowledgeBase, &s.KnowledgeBaseFiles,
		&s.GreetingMessage, &s.TrialAmountSGD, &s.ManagedBy1Hero, &s.BookingHeroImageURL, &s.BookingHeroVideoURL); err != nil {
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

// SearchKnowledgeChunks performs hybrid retrieval: vector similarity + BM25 full-text search,
// fused with Reciprocal Rank Fusion (RRF). Returns top-K deduplicated chunks.
// Falls back to pure vector search if the FTS column is not yet available.
// platform filters to chunks tagged for that channel plus "all" chunks; pass "" or "all" to skip filtering.
func (r *Repo) SearchKnowledgeChunks(ctx context.Context, studioID uuid.UUID, queryEmbedding []float32, platform string, limit int) ([]string, error) {
	embStr := FormatVectorAsString(queryEmbedding)
	candidateN := limit * 5
	if platform == "" {
		platform = "all"
	}

	rows, err := r.pool.Query(ctx, `
		WITH vector_ranked AS (
			SELECT content, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			ORDER BY embedding <=> $2::vector
			LIMIT $5
		),
		fts_ranked AS (
			SELECT content, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			  AND content_tsv @@ plainto_tsquery('english', $4)
			ORDER BY ts_rank(content_tsv, plainto_tsquery('english', $4)) DESC
			LIMIT $5
		),
		fused AS (
			SELECT content,
				COALESCE(v.rrf, 0) + COALESCE(f.rrf, 0) AS score
			FROM (
				SELECT content, 1.0 / (60 + rank) AS rrf FROM vector_ranked
			) v
			FULL OUTER JOIN (
				SELECT content, 1.0 / (60 + rank) AS rrf FROM fts_ranked
			) f USING (content)
		)
		SELECT content FROM fused
		ORDER BY score DESC
		LIMIT $6
	`, studioID, embStr, platform, "", candidateN, limit)

	if err != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT content
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
	var results []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		if !seen[content] {
			seen[content] = true
			results = append(results, content)
		}
	}
	return results, rows.Err()
}

// SearchKnowledgeChunksHybrid is the same as SearchKnowledgeChunks but accepts
// an explicit query string for BM25 (used when the caller has a plain-text query).
// platform filters chunks for the conversation's channel; "all" returns everything.
func (r *Repo) SearchKnowledgeChunksHybrid(ctx context.Context, studioID uuid.UUID, queryEmbedding []float32, queryText string, platform string, limit int) ([]string, error) {
	embStr := FormatVectorAsString(queryEmbedding)
	candidateN := limit * 5
	if platform == "" {
		platform = "all"
	}

	rows, err := r.pool.Query(ctx, `
		WITH vector_ranked AS (
			SELECT content, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			ORDER BY embedding <=> $2::vector
			LIMIT $5
		),
		fts_ranked AS (
			SELECT content, ROW_NUMBER() OVER () AS rank
			FROM studio_knowledge_chunks
			WHERE studio_id = $1
			  AND ($3 = 'all' OR platform IN ('all', $3))
			  AND content_tsv @@ plainto_tsquery('english', $4)
			ORDER BY ts_rank(content_tsv, plainto_tsquery('english', $4)) DESC
			LIMIT $5
		),
		fused AS (
			SELECT content,
				COALESCE(v.rrf, 0) + COALESCE(f.rrf, 0) AS score
			FROM (
				SELECT content, 1.0 / (60 + rank) AS rrf FROM vector_ranked
			) v
			FULL OUTER JOIN (
				SELECT content, 1.0 / (60 + rank) AS rrf FROM fts_ranked
			) f USING (content)
		)
		SELECT content FROM fused
		ORDER BY score DESC
		LIMIT $6
	`, studioID, embStr, platform, queryText, candidateN, limit)

	if err != nil {
		return r.SearchKnowledgeChunks(ctx, studioID, queryEmbedding, platform, limit)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var results []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("scan chunk hybrid: %w", err)
		}
		if !seen[content] {
			seen[content] = true
			results = append(results, content)
		}
	}
	return results, rows.Err()
}
