package studios

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/gemini"
	"github.com/projectx/api/internal/integrations/groq"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/platform/httpx"
)

// aiProviderKeyGetters reads the studio's saved key for provider, so the
// test/add-model handler doesn't need a second provider->column switch.
var aiProviderKeyGetters = map[string]func(*Studio) string{
	"gemini": func(s *Studio) string { return s.GeminiAPIKey },
	"groq":   func(s *Studio) string { return s.GroqAPIKey },
	"claude": func(s *Studio) string { return s.ClaudeAPIKey },
}

func isKnownProvider(provider string) bool {
	_, ok := aiProviderKeyGetters[provider]
	return ok
}

// keySuffix returns the last 4 characters of key for display (e.g. "ends
// in cHwK"), or "" if there's no key to show one for. Provider keys are
// stored plaintext today (see UpdateAIProviderKey), so this is a plain
// substring — never derived from the encrypted form.
func keySuffix(key string) string {
	if len(key) < 4 {
		return ""
	}
	return key[len(key)-4:]
}

const testPingPrompt = "Reply with the single word: pong"

// testProviderModel makes one trivial live request to provider/model with
// apiKey, used both by PostAIModel (add+test a specific model) and
// PostAITestKey (verify a key works at all, using a representative model).
func testProviderModel(ctx context.Context, provider, claudeAPIURL, apiKey, model string) error {
	switch provider {
	case "groq":
		client := groq.New(apiKey)
		_, err := client.GenerateReply(ctx, testPingPrompt, model)
		return err
	case "gemini":
		_, err := gemini.New().GenerateReplyForModel(ctx, apiKey, model, testPingPrompt)
		return err
	case "claude":
		client, err := claude.New(claudeAPIURL, apiKey)
		if err != nil {
			return err
		}
		if client == nil {
			return errors.New("claude client not configured")
		}
		_, err = client.GenerateReplyForModel(ctx, testPingPrompt, model)
		return err
	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
}

// aiModelResponse is the wire shape for one row in the checkbox list —
// covers both a persisted studio_ai_models row and a virtual (unsaved)
// default shown so the admin can see what's actually running.
type aiModelResponse struct {
	ID            string `json:"id,omitempty"`
	ModelName     string `json:"modelName"`
	Enabled       bool   `json:"enabled"`
	IsDefault     bool   `json:"isDefault"`
	LastTestedAt  string `json:"lastTestedAt,omitempty"`
	LastTestOK    *bool  `json:"lastTestOk,omitempty"`
	LastTestError string `json:"lastTestError,omitempty"`
}

type aiProviderResponse struct {
	Provider  string            `json:"provider"`
	HasAPIKey bool              `json:"hasApiKey"`
	KeySuffix string            `json:"keySuffix,omitempty"`
	Models    []aiModelResponse `json:"models"`
}

// GetAIModels godoc
//
//	@Summary		List AI provider config
//	@Description	Per-provider (groq/gemini/claude) API key status and model checklist for this studio — persisted models the studio has added/tested, or the platform's implicit default model(s) shown unchecked when the studio hasn't configured any yet.
//	@Tags			AI Assistant Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path		string	true	"Studio ID"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/studios/{studioId}/ai-models [get]
func (h *Handler) GetAIModels(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	providers := []string{"groq", "gemini", "claude"}
	out := make([]aiProviderResponse, 0, len(providers))
	for _, provider := range providers {
		rows, err := h.llmRepo.ListStudioModels(r.Context(), studioID, llm.ProviderName(provider))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}

		var models []aiModelResponse
		if len(rows) == 0 {
			for _, name := range llm.DefaultModels[llm.ProviderName(provider)] {
				models = append(models, aiModelResponse{ModelName: name, Enabled: false, IsDefault: true})
			}
		} else {
			for _, m := range rows {
				resp := aiModelResponse{
					ID:            m.ID.String(),
					ModelName:     m.ModelName,
					Enabled:       m.Enabled,
					LastTestOK:    m.LastTestOK,
					LastTestError: m.LastTestError,
				}
				if m.LastTestedAt != nil {
					resp.LastTestedAt = m.LastTestedAt.Format(time.RFC3339)
				}
				models = append(models, resp)
			}
		}

		key := aiProviderKeyGetters[provider](studio)
		out = append(out, aiProviderResponse{
			Provider:  provider,
			HasAPIKey: key != "",
			KeySuffix: keySuffix(key),
			Models:    models,
		})
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"providers": out})
}

type putAIProviderKeyReq struct {
	APIKey string `json:"apiKey"`
}

// PutAIProviderKey godoc
//
//	@Summary		Save an AI provider's API key
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string					true	"Studio ID"
//	@Param			provider	path	string					true	"groq | gemini | claude"
//	@Param			body		body	putAIProviderKeyReq	true	"API key"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/studios/{studioId}/ai-models/{provider}/key [put]
func (h *Handler) PutAIProviderKey(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	provider := chi.URLParam(r, "provider")
	if !isKnownProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req putAIProviderKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}
	if err := h.svc.repo.UpdateAIProviderKey(r.Context(), studioID, provider, req.APIKey); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PostAITestKey godoc
//
//	@Summary		Test an AI provider's API key
//	@Description	Saves apiKey (if provided) and sends one trivial live request using an already-enabled model for this provider, or the platform default if none is enabled yet — verifies the key works without requiring the admin to add a specific model first.
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string					true	"Studio ID"
//	@Param			provider	path	string					true	"groq | gemini | claude"
//	@Param			body		body	putAIProviderKeyReq	true	"API key (omit to test the already-saved key)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		422			{object}	httpx.ErrorResponse	"the live test failed"
//	@Router			/studios/{studioId}/ai-models/{provider}/test-key [post]
func (h *Handler) PostAITestKey(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	provider := chi.URLParam(r, "provider")
	if !isKnownProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req putAIProviderKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = aiProviderKeyGetters[provider](studio)
	}
	if apiKey == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}

	models, err := h.llmRepo.EnabledModelsForStudio(r.Context(), studioID, llm.ProviderName(provider))
	if err != nil || len(models) == 0 {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if testErr := testProviderModel(testCtx, provider, h.claudeAPIURL, apiKey, models[0]); testErr != nil {
		httpx.WriteValidationError(w, map[string]string{"apiKey": testErr.Error()})
		return
	}

	// Only touch storage after a successful test — save the key if a new one was submitted.
	if strings.TrimSpace(req.APIKey) != "" {
		if err := h.svc.repo.UpdateAIProviderKey(r.Context(), studioID, provider, req.APIKey); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "testedModel": models[0]})
}

type postAIModelReq struct {
	ModelName string `json:"modelName"`
}

// PostAIModel godoc
//
//	@Summary		Add and live-test a model for a provider
//	@Description	Sends a trivial live request to the provider using the studio's saved key. On success the model is persisted and enabled; on failure nothing is written.
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string			true	"Studio ID"
//	@Param			provider	path	string			true	"groq | gemini | claude"
//	@Param			body		body	postAIModelReq	true	"Model name"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		422			{object}	httpx.ErrorResponse	"the live test failed"
//	@Router			/studios/{studioId}/ai-models/{provider}/models [post]
func (h *Handler) PostAIModel(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	provider := chi.URLParam(r, "provider")
	if !isKnownProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req postAIModelReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	modelName := strings.TrimSpace(req.ModelName)
	if modelName == "" {
		httpx.WriteValidationError(w, map[string]string{"modelName": "required"})
		return
	}

	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	apiKey := aiProviderKeyGetters[provider](studio)
	if apiKey == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "save an API key for this provider first"})
		return
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if testErr := testProviderModel(testCtx, provider, h.claudeAPIURL, apiKey, modelName); testErr != nil {
		httpx.WriteValidationError(w, map[string]string{"modelName": testErr.Error()})
		return
	}

	m, err := h.llmRepo.UpsertStudioModelResult(r.Context(), studioID, llm.ProviderName(provider), modelName, true, "")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":        m.ID.String(),
		"modelName": m.ModelName,
		"enabled":   m.Enabled,
	})
}

type patchAIModelReq struct {
	Enabled bool `json:"enabled"`
}

// PatchAIModel godoc
//
//	@Summary		Enable or disable an already-tested model
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string			true	"Studio ID"
//	@Param			provider	path	string			true	"groq | gemini | claude"
//	@Param			modelId		path	string			true	"Model ID"
//	@Param			body		body	patchAIModelReq	true	"Enabled state"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/studios/{studioId}/ai-models/{provider}/models/{modelId} [patch]
func (h *Handler) PatchAIModel(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	modelID, err := uuid.Parse(chi.URLParam(r, "modelId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid model id")
		return
	}
	var req patchAIModelReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := h.llmRepo.SetStudioModelEnabled(r.Context(), studioID, modelID, req.Enabled); err != nil {
		if errors.Is(err, llm.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "model not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type patchAIModelOrderReq struct {
	ModelIDs []string `json:"modelIds"`
}

// PatchAIModelOrder godoc
//
//	@Summary		Reorder a provider's models
//	@Description	Sets the try-order for provider's models — modelIds[0] is tried first by the waterfall, modelIds[1] is the fallback, and so on.
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string					true	"Studio ID"
//	@Param			provider	path	string					true	"groq | gemini | claude"
//	@Param			body		body	patchAIModelOrderReq	true	"Model IDs in the desired try-order"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/studios/{studioId}/ai-models/{provider}/order [patch]
func (h *Handler) PatchAIModelOrder(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	provider := chi.URLParam(r, "provider")
	if !isKnownProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req patchAIModelOrderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ids := make([]uuid.UUID, 0, len(req.ModelIDs))
	for _, s := range req.ModelIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			httpx.WriteValidationError(w, map[string]string{"modelIds": "invalid model id"})
			return
		}
		ids = append(ids, id)
	}
	if err := h.llmRepo.SetStudioModelOrder(r.Context(), studioID, llm.ProviderName(provider), ids); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DeleteAIModel godoc
//
//	@Summary		Remove a model a studio added
//	@Tags			AI Assistant Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID"
//	@Param			provider	path	string	true	"groq | gemini | claude"
//	@Param			modelId		path	string	true	"Model ID"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		404			{object}	httpx.ErrorResponse
//	@Router			/studios/{studioId}/ai-models/{provider}/models/{modelId} [delete]
func (h *Handler) DeleteAIModel(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	modelID, err := uuid.Parse(chi.URLParam(r, "modelId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid model id")
		return
	}
	if err := h.llmRepo.DeleteStudioModel(r.Context(), studioID, modelID); err != nil {
		if errors.Is(err, llm.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "model not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
