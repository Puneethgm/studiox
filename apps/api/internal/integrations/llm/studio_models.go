package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Default model names for each provider's fallback list below — these were
// previously imported from the old standalone groq/claude/gemini HTTP
// client packages (internal/integrations/{groq,claude,gemini}), which
// Phase 3 of the LLM gateway migration removed (every real call now goes
// through internal/integrations/bifrost). Only the model-name constants
// those packages defined were still in use; moved here rather than lost.
const (
	groqModel8B       = "qwen/qwen3.8-27b"
	groqModel70B      = "openai/gpt-oss-120b"
	claudeDefaultModel = "claude-haiku-4-5-20251001"
)

var geminiDefaultModels = []string{"gemini-flash-latest", "gemini-flash-lite-latest"}

// PlatformAIModel is one row of platform_ai_models — a model a superadmin
// has enabled (or previously tested and disabled) for a provider,
// platform-wide. Model selection used to be per-studio (studio_ai_models);
// it's centralized now, same as the provider keys themselves (see
// studios.Repo.GetPlatformAIProviderKey), so every studio gets the same,
// superadmin-vetted model list instead of quality varying by what each
// studio happened to configure. Distinct from TaskConfig/ai_task_configs,
// which is one-model-per-named-task, not a provider-wide ordered list.
type PlatformAIModel struct {
	ID            uuid.UUID
	Provider      ProviderName
	ModelName     string
	Enabled       bool
	SortOrder     int
	LastTestedAt  *time.Time
	LastTestOK    *bool
	LastTestError string
	CreatedAt     time.Time
}

// DefaultModels is the implicit fallback used for a provider when the
// superadmin has configured zero *enabled* models for it — preserves the
// original hardcoded behavior until the AI Provider Settings page
// (superadmin, /admin/settings) has been used at least once.
var DefaultModels = map[ProviderName][]string{
	ProviderGroq:   {groqModel8B, groqModel70B},
	ProviderGemini: geminiDefaultModels,
	ProviderClaude: {claudeDefaultModel},
	ProviderOpenAI: {"gpt-4o-mini"},
}

// ListModels returns every model a superadmin has ever added for provider
// (enabled or not), ordered for waterfall use.
func (r *Repo) ListModels(ctx context.Context, provider ProviderName) ([]PlatformAIModel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, provider, model_name, enabled, sort_order,
		       last_tested_at, last_test_ok, last_test_error, created_at
		FROM platform_ai_models
		WHERE provider = $1
		ORDER BY sort_order, created_at
	`, provider)
	if err != nil {
		return nil, fmt.Errorf("list platform ai models: %w", err)
	}
	defer rows.Close()

	var out []PlatformAIModel
	for rows.Next() {
		var m PlatformAIModel
		var lastTestError *string
		if err := rows.Scan(&m.ID, &m.Provider, &m.ModelName, &m.Enabled, &m.SortOrder,
			&m.LastTestedAt, &m.LastTestOK, &lastTestError, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan platform ai model: %w", err)
		}
		if lastTestError != nil {
			m.LastTestError = *lastTestError
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertModelResult persists a live test's outcome for (provider,
// modelName), enabling the model on success. Called only after a successful
// test — a failed test never writes a row (the caller returns the error to
// the superadmin instead).
func (r *Repo) UpsertModelResult(ctx context.Context, provider ProviderName, modelName string, ok bool, testErr string) (*PlatformAIModel, error) {
	var errPtr *string
	if testErr != "" {
		errPtr = &testErr
	}
	var m PlatformAIModel
	var lastTestError *string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO platform_ai_models (provider, model_name, enabled, last_tested_at, last_test_ok, last_test_error)
		VALUES ($1, $2, true, now(), $3, $4)
		ON CONFLICT (provider, model_name) DO UPDATE
		SET enabled = true,
		    last_tested_at = now(),
		    last_test_ok = EXCLUDED.last_test_ok,
		    last_test_error = EXCLUDED.last_test_error
		RETURNING id, provider, model_name, enabled, sort_order,
		          last_tested_at, last_test_ok, last_test_error, created_at
	`, provider, modelName, ok, errPtr).Scan(
		&m.ID, &m.Provider, &m.ModelName, &m.Enabled, &m.SortOrder,
		&m.LastTestedAt, &m.LastTestOK, &lastTestError, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert platform ai model: %w", err)
	}
	if lastTestError != nil {
		m.LastTestError = *lastTestError
	}
	return &m, nil
}

// SetModelEnabled toggles an already-persisted model without a live call —
// used to disable a model, or re-enable one that was already tested
// successfully before.
func (r *Repo) SetModelEnabled(ctx context.Context, modelID uuid.UUID, enabled bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE platform_ai_models SET enabled = $2 WHERE id = $1
	`, modelID, enabled)
	if err != nil {
		return fmt.Errorf("set platform ai model enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetModelOrder persists a new try-order for provider's models —
// orderedIDs[0] becomes the first one the waterfall tries, orderedIDs[1]
// the fallback, and so on.
func (r *Repo) SetModelOrder(ctx context.Context, provider ProviderName, orderedIDs []uuid.UUID) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	for i, id := range orderedIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE platform_ai_models SET sort_order = $1
			WHERE id = $2 AND provider = $3
		`, i, id, provider); err != nil {
			return fmt.Errorf("set platform ai model order: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// DeleteModel removes a model a superadmin added.
func (r *Repo) DeleteModel(ctx context.Context, modelID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM platform_ai_models WHERE id = $1
	`, modelID)
	if err != nil {
		return fmt.Errorf("delete platform ai model: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnabledModels returns the ordered list of model names every studio may
// use for provider — the single call site every reply/summary code path
// should use instead of a hardcoded model constant. Falls back to
// DefaultModels when the superadmin has configured zero *enabled* models
// for provider, so a provider never goes fully dark because of this
// feature.
func (r *Repo) EnabledModels(ctx context.Context, provider ProviderName) ([]string, error) {
	rows, err := r.ListModels(ctx, provider)
	if err != nil {
		return nil, err
	}
	var enabled []string
	for _, m := range rows {
		if m.Enabled {
			enabled = append(enabled, m.ModelName)
		}
	}
	if len(enabled) == 0 {
		return DefaultModels[provider], nil
	}
	return enabled, nil
}
