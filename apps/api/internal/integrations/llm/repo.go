package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/secrets"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

func NewRepo(pool *pgxpool.Pool, cipher *secrets.Cipher) *Repo {
	return &Repo{pool: pool, cipher: cipher}
}

func (r *Repo) GetTaskConfig(ctx context.Context, purpose string) (*TaskConfig, error) {
	var cfg TaskConfig
	var apiKeyEnc *string
	err := r.pool.QueryRow(ctx, `
		SELECT purpose, provider, model, api_key_enc, updated_at
		FROM ai_task_configs WHERE purpose = $1
	`, purpose).Scan(&cfg.Purpose, &cfg.Provider, &cfg.Model, &apiKeyEnc, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task config: %w", err)
	}
	if apiKeyEnc != nil {
		cfg.APIKeyEnc = *apiKeyEnc
	}
	return &cfg, nil
}

// DecryptAPIKey returns the plaintext override key for cfg, or "" if none
// was set (the caller should then use that provider's platform-default key).
func (r *Repo) DecryptAPIKey(cfg *TaskConfig) (string, error) {
	if cfg.APIKeyEnc == "" {
		return "", nil
	}
	return r.cipher.Decrypt(cfg.APIKeyEnc)
}

// UpsertTaskConfig saves a super-admin's provider/model choice for purpose.
// apiKey is the plaintext override to encrypt and store — pass "" to leave
// the existing override (if any) untouched, matching the "blank means don't
// change" convention used by the studio Settings forms elsewhere.
func (r *Repo) UpsertTaskConfig(ctx context.Context, purpose string, provider ProviderName, model, apiKey string, updatedBy uuid.UUID) error {
	var encPtr *string
	if apiKey != "" {
		enc, err := r.cipher.Encrypt(apiKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		encPtr = &enc
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_task_configs (purpose, provider, model, api_key_enc, updated_by)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (purpose) DO UPDATE
		SET provider = EXCLUDED.provider,
		    model = EXCLUDED.model,
		    api_key_enc = COALESCE(EXCLUDED.api_key_enc, ai_task_configs.api_key_enc),
		    updated_by = EXCLUDED.updated_by,
		    updated_at = now()
	`, purpose, provider, model, encPtr, updatedBy)
	if err != nil {
		return fmt.Errorf("upsert task config: %w", err)
	}
	return nil
}
