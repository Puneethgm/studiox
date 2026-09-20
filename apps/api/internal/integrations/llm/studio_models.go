package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/gemini"
	"github.com/projectx/api/internal/integrations/groq"
)

// StudioAIModel is one row of studio_ai_models — a model a studio has
// enabled (or previously tested and disabled) for a provider. Distinct from
// TaskConfig/ai_task_configs, which is platform-scoped and one-model-per-
// named-task; this is per-studio and a provider can have several enabled
// models (e.g. Groq's small-then-large fallback order).
type StudioAIModel struct {
	ID            uuid.UUID
	StudioID      uuid.UUID
	Provider      ProviderName
	ModelName     string
	Enabled       bool
	SortOrder     int
	LastTestedAt  *time.Time
	LastTestOK    *bool
	LastTestError string
	CreatedAt     time.Time
}

// DefaultModels is the implicit fallback used for a provider when a studio
// has configured zero *enabled* models for it — preserves today's
// hardcoded behavior for every studio that never opens the AI Assistant
// settings page, and for a studio that has disabled every model it added.
var DefaultModels = map[ProviderName][]string{
	ProviderGroq:   {groq.Model8B, groq.Model70B},
	ProviderGemini: gemini.Models,
	ProviderClaude: {claude.DefaultModel},
}

// ListStudioModels returns every model a studio has ever added for
// provider (enabled or not), ordered for waterfall use.
func (r *Repo) ListStudioModels(ctx context.Context, studioID uuid.UUID, provider ProviderName) ([]StudioAIModel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, studio_id, provider, model_name, enabled, sort_order,
		       last_tested_at, last_test_ok, last_test_error, created_at
		FROM studio_ai_models
		WHERE studio_id = $1 AND provider = $2
		ORDER BY sort_order, created_at
	`, studioID, provider)
	if err != nil {
		return nil, fmt.Errorf("list studio ai models: %w", err)
	}
	defer rows.Close()

	var out []StudioAIModel
	for rows.Next() {
		var m StudioAIModel
		var lastTestError *string
		if err := rows.Scan(&m.ID, &m.StudioID, &m.Provider, &m.ModelName, &m.Enabled, &m.SortOrder,
			&m.LastTestedAt, &m.LastTestOK, &lastTestError, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan studio ai model: %w", err)
		}
		if lastTestError != nil {
			m.LastTestError = *lastTestError
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertStudioModelResult persists a live test's outcome for
// (studioID, provider, modelName), enabling the model on success. Called
// only after a successful test — a failed test never writes a row (the
// caller returns the error to the admin instead).
func (r *Repo) UpsertStudioModelResult(ctx context.Context, studioID uuid.UUID, provider ProviderName, modelName string, ok bool, testErr string) (*StudioAIModel, error) {
	var errPtr *string
	if testErr != "" {
		errPtr = &testErr
	}
	var m StudioAIModel
	var lastTestError *string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO studio_ai_models (studio_id, provider, model_name, enabled, last_tested_at, last_test_ok, last_test_error)
		VALUES ($1, $2, $3, true, now(), $4, $5)
		ON CONFLICT (studio_id, provider, model_name) DO UPDATE
		SET enabled = true,
		    last_tested_at = now(),
		    last_test_ok = EXCLUDED.last_test_ok,
		    last_test_error = EXCLUDED.last_test_error
		RETURNING id, studio_id, provider, model_name, enabled, sort_order,
		          last_tested_at, last_test_ok, last_test_error, created_at
	`, studioID, provider, modelName, ok, errPtr).Scan(
		&m.ID, &m.StudioID, &m.Provider, &m.ModelName, &m.Enabled, &m.SortOrder,
		&m.LastTestedAt, &m.LastTestOK, &lastTestError, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert studio ai model: %w", err)
	}
	if lastTestError != nil {
		m.LastTestError = *lastTestError
	}
	return &m, nil
}

// SetStudioModelEnabled toggles an already-persisted model without a live
// call — used to disable a model, or re-enable one that was already tested
// successfully before.
func (r *Repo) SetStudioModelEnabled(ctx context.Context, studioID, modelID uuid.UUID, enabled bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE studio_ai_models SET enabled = $3 WHERE id = $1 AND studio_id = $2
	`, modelID, studioID, enabled)
	if err != nil {
		return fmt.Errorf("set studio ai model enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStudioModelOrder persists a new try-order for provider's models —
// orderedIDs[0] becomes the first one the waterfall tries, orderedIDs[1]
// the fallback, and so on. Lets a studio admin pick which model is
// "primary" vs "fallback" for a provider from the AI Assistant page,
// instead of it being fixed at whatever order models were added in.
func (r *Repo) SetStudioModelOrder(ctx context.Context, studioID uuid.UUID, provider ProviderName, orderedIDs []uuid.UUID) error {
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
			UPDATE studio_ai_models SET sort_order = $1
			WHERE id = $2 AND studio_id = $3 AND provider = $4
		`, i, id, studioID, provider); err != nil {
			return fmt.Errorf("set studio ai model order: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// DeleteStudioModel removes a custom model a studio added.
func (r *Repo) DeleteStudioModel(ctx context.Context, studioID, modelID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM studio_ai_models WHERE id = $1 AND studio_id = $2
	`, modelID, studioID)
	if err != nil {
		return fmt.Errorf("delete studio ai model: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnabledModelsForStudio returns the ordered list of model names a studio
// may use for provider — the single call site every reply/summary code
// path should use instead of a hardcoded model constant. Falls back to
// DefaultModels when the studio has zero *enabled* models for provider
// (never configured it, or disabled everything it added), so a provider
// never goes fully dark because of this feature.
func (r *Repo) EnabledModelsForStudio(ctx context.Context, studioID uuid.UUID, provider ProviderName) ([]string, error) {
	rows, err := r.ListStudioModels(ctx, studioID, provider)
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
