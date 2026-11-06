package studios

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/integrations/mistral"
	"github.com/projectx/api/internal/platform/httpx"
	"github.com/projectx/api/internal/platform/riskalert"
	"github.com/projectx/api/internal/platform/secaudit"
)

// chatReplyProviders are the providers centrally managed from the
// superadmin AI Provider Settings page (/admin/settings) — Groq/Gemini/
// Claude, each routed through the embedded Bifrost client. Mistral stays a
// per-studio key (OCR only, internal/studios/knowledge_base_ocr.go — not a
// chat-reply provider, so it isn't part of this centralized list) and keeps
// its own studio-scoped routes below.
var chatReplyProviders = map[string]bifrost.ProviderName{
	"groq":   bifrost.ProviderGroq,
	"gemini": bifrost.ProviderGemini,
	"claude": bifrost.ProviderAnthropic,
	"openai": bifrost.ProviderOpenAI,
}

func isKnownChatProvider(provider string) bool {
	_, ok := chatReplyProviders[provider]
	return ok
}

const testPingPrompt = "Reply with the single word: pong"

// testPlatformProviderModel makes one trivial live Bifrost request to
// provider/model with apiKey — used both by PostPlatformAIModel (add+test a
// specific model) and PostPlatformAITestKey (verify a key works at all,
// using a representative model).
func testPlatformProviderModel(ctx context.Context, bifrostClient *bifrost.Client, provider, apiKey, model string) error {
	bfProvider, ok := chatReplyProviders[provider]
	if !ok {
		return fmt.Errorf("unsupported provider %q", provider)
	}
	if bifrostClient == nil {
		return errors.New("bifrost gateway not configured")
	}
	_, err := bifrostClient.Complete(ctx, bifrost.Request{
		Provider:  bfProvider,
		Model:     model,
		DirectKey: apiKey,
		Prompt:    testPingPrompt,
		// Reasoning models (e.g. Groq's openai/gpt-oss-120b) spend tokens on
		// hidden reasoning before any visible reply text — 16 was enough for
		// non-reasoning models but left zero budget for the actual "pong",
		// so Bifrost reported it as an empty response. 128 gives reasoning
		// models room to think and still answer.
		MaxTokens: 128,
	})
	return err
}

// fingerprintSecret returns a short, non-reversible fingerprint of key
// (sha256, first 8 hex chars) so two keys can be told apart without any
// character of either being derivable from the result. Replaces the
// previous keySuffix() last-4-characters reveal — see
// openspec/changes/platform-security-hardening, task 0.2.
func fingerprintSecret(key string) string {
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:8]
}

// aiModelResponse is the wire shape for one row in the checkbox list —
// covers both a persisted platform_ai_models row and a virtual (unsaved)
// default shown so the superadmin can see what's actually running.
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
	Provider       string            `json:"provider"`
	HasAPIKey      bool              `json:"hasApiKey"`
	KeyFingerprint string            `json:"keyFingerprint,omitempty"`
	KeyStatus      string            `json:"keyStatus,omitempty"` // "active" | "disabled" | "failing", "" if no key ever saved
	Models         []aiModelResponse `json:"models"`
}

// GetPlatformAIModels godoc
//
//	@Summary		List centralized AI provider config
//	@Description	Per-provider (groq/gemini/claude/openai) API key status and model checklist, shared by every studio — persisted models the superadmin has added/tested, or the platform's implicit default model(s) shown unchecked when none has been configured yet.
//	@Tags			AI Provider Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	map[string]any
//	@Failure		500	{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers [get]
func (h *Handler) GetPlatformAIModels(w http.ResponseWriter, r *http.Request) {
	providers := []string{"groq", "gemini", "claude", "openai"}
	out := make([]aiProviderResponse, 0, len(providers))
	for _, provider := range providers {
		rows, err := h.llmRepo.ListModels(r.Context(), llm.ProviderName(provider))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}

		models := []aiModelResponse{}
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

		hasKey, fingerprint, err := h.svc.repo.GetPlatformAIProviderKeyFingerprint(r.Context(), provider)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		keyStatus, err := h.svc.repo.GetPlatformAIProviderKeyStatus(r.Context(), provider)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		out = append(out, aiProviderResponse{
			Provider:       provider,
			HasAPIKey:      hasKey,
			KeyFingerprint: fingerprint,
			KeyStatus:      keyStatus,
			Models:         models,
		})
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"providers": out})
}

type putPlatformAIProviderKeyReq struct {
	APIKey string `json:"apiKey"`
}

// PutPlatformAIProviderKey godoc
//
//	@Summary		Save a centralized AI provider's API key
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string							true	"groq | gemini | claude | openai"
//	@Param			body		body	putPlatformAIProviderKeyReq	true	"API key"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/{provider}/key [put]
func (h *Handler) PutPlatformAIProviderKey(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req putPlatformAIProviderKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.svc.repo.SetPlatformAIProviderKey(r.Context(), provider, req.APIKey, &c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), nil, "ai_provider_key.set", secaudit.OutcomeSuccess, map[string]any{"provider": provider})
	if riskalert.SecretChanged() {
		h.sendRiskAlert("Platform AI provider key changed", fmt.Sprintf("The %s provider key was set/replaced by a super_admin.", provider))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PostPlatformAIProviderKeyRotate godoc
//
//	@Summary		Rotate a centralized AI provider's API key
//	@Description	Replaces the stored key with a freshly re-encrypted envelope (new data key) and records the change as a "rotated" audit action, distinct from the first save.
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string							true	"groq | gemini | claude | openai"
//	@Param			body		body	putPlatformAIProviderKeyReq	true	"New API key"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/{provider}/key/rotate [post]
func (h *Handler) PostPlatformAIProviderKeyRotate(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req putPlatformAIProviderKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.svc.repo.RotatePlatformAIProviderKey(r.Context(), provider, req.APIKey, &c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), nil, "ai_provider_key.rotated", secaudit.OutcomeSuccess, map[string]any{"provider": provider})
	if riskalert.SecretChanged() {
		h.sendRiskAlert("Platform AI provider key rotated", fmt.Sprintf("The %s provider key was rotated by a super_admin.", provider))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PatchPlatformAIProviderKeyStatus godoc
//
//	@Summary		Disable or re-enable a centralized AI provider's key
//	@Description	A disabled key is treated as "not configured" by every caller (GetPlatformAIProviderKey) until re-enabled — the ciphertext is kept, not deleted, so re-enabling doesn't require re-entering the key.
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string	true	"groq | gemini | claude | openai"
//	@Param			body		body	patchPlatformAIProviderKeyStatusReq	true	"status: active | disabled"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/{provider}/key/status [patch]
func (h *Handler) PatchPlatformAIProviderKeyStatus(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req patchPlatformAIProviderKeyStatusReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Status != "active" && req.Status != "disabled" {
		httpx.WriteValidationError(w, map[string]string{"status": "must be \"active\" or \"disabled\""})
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.svc.repo.SetPlatformAIProviderKeyStatus(r.Context(), provider, req.Status, &c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type patchPlatformAIProviderKeyStatusReq struct {
	Status string `json:"status"`
}

// GetAIAlertThresholds godoc
//
//	@Summary		Get the AI alert thresholds
//	@Description	Failure-rate-spike %, its rolling window, and the daily spend budget — the trigger points for the two threshold-based AI alerts.
//	@Tags			AI Provider Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	studios.AIAlertThresholds
//	@Router			/admin/ai-providers/alert-thresholds [get]
func (h *Handler) GetAIAlertThresholds(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.repo.GetAIAlertThresholds(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

// PutAIAlertThresholds godoc
//
//	@Summary		Set the AI alert thresholds
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body	studios.AIAlertThresholds	true	"thresholds"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/alert-thresholds [put]
func (h *Handler) PutAIAlertThresholds(w http.ResponseWriter, r *http.Request) {
	var req AIAlertThresholds
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.svc.repo.SetAIAlertThresholds(r.Context(), req, &c.UserID); err != nil {
		httpx.WriteValidationError(w, map[string]string{"thresholds": err.Error()})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GetAIAlerts godoc
//
//	@Summary		List recent AI alerts
//	@Description	Most recent platform_alerts rows — all-providers-failed, a key failing, provider failure-rate spikes, daily spend over budget.
//	@Tags			AI Provider Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	map[string]any
//	@Router			/admin/ai-providers/alerts [get]
func (h *Handler) GetAIAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.svc.repo.ListRecentPlatformAlerts(r.Context(), 50)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"alerts": alerts})
}

// PostPlatformAITestKey godoc
//
//	@Summary		Test a centralized AI provider's API key
//	@Description	Saves apiKey (if provided) and sends one trivial live request using an already-enabled model for this provider, or the platform default if none is enabled yet — verifies the key works without requiring the superadmin to add a specific model first.
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string							true	"groq | gemini | claude | openai"
//	@Param			body		body	putPlatformAIProviderKeyReq	true	"API key (omit to test the already-saved key)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		422			{object}	httpx.ErrorResponse	"the live test failed"
//	@Router			/admin/ai-providers/{provider}/test-key [post]
func (h *Handler) PostPlatformAITestKey(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req putPlatformAIProviderKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		var err error
		apiKey, err = h.svc.repo.GetPlatformAIProviderKey(r.Context(), provider)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	if apiKey == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}

	models, err := h.llmRepo.EnabledModels(r.Context(), llm.ProviderName(provider))
	if err != nil || len(models) == 0 {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	testModel := models[0]

	testCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if testErr := testPlatformProviderModel(testCtx, h.bifrostClient, provider, apiKey, testModel); testErr != nil {
		httpx.WriteValidationError(w, map[string]string{"apiKey": testErr.Error()})
		return
	}

	if strings.TrimSpace(req.APIKey) != "" {
		c := identity.MustClaims(r.Context())
		if err := h.svc.repo.SetPlatformAIProviderKey(r.Context(), provider, req.APIKey, &c.UserID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true, "testedModel": testModel})
}

type postPlatformAIModelReq struct {
	ModelName string `json:"modelName"`
}

// PostPlatformAIModel godoc
//
//	@Summary		Add and live-test a model for a provider
//	@Description	Sends a trivial live request using the centrally-configured key. On success the model is persisted and enabled for every studio; on failure nothing is written.
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string					true	"groq | gemini | claude | openai"
//	@Param			body		body	postPlatformAIModelReq	true	"Model name"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		422			{object}	httpx.ErrorResponse	"the live test failed"
//	@Router			/admin/ai-providers/{provider}/models [post]
func (h *Handler) PostPlatformAIModel(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req postPlatformAIModelReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	modelName := strings.TrimSpace(req.ModelName)
	if modelName == "" {
		httpx.WriteValidationError(w, map[string]string{"modelName": "required"})
		return
	}

	apiKey, err := h.svc.repo.GetPlatformAIProviderKey(r.Context(), provider)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if apiKey == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "save an API key for this provider first"})
		return
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if testErr := testPlatformProviderModel(testCtx, h.bifrostClient, provider, apiKey, modelName); testErr != nil {
		httpx.WriteValidationError(w, map[string]string{"modelName": testErr.Error()})
		return
	}

	m, err := h.llmRepo.UpsertModelResult(r.Context(), llm.ProviderName(provider), modelName, true, "")
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			httpx.WriteValidationError(w, map[string]string{"provider": "this provider isn't enabled for custom models yet"})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":        m.ID.String(),
		"modelName": m.ModelName,
		"enabled":   m.Enabled,
	})
}

type patchPlatformAIModelReq struct {
	Enabled bool `json:"enabled"`
}

// PatchPlatformAIModel godoc
//
//	@Summary		Enable or disable an already-tested model
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			modelId	path	string					true	"Model ID"
//	@Param			body	body	patchPlatformAIModelReq	true	"Enabled state"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/models/{modelId} [patch]
func (h *Handler) PatchPlatformAIModel(w http.ResponseWriter, r *http.Request) {
	modelID, err := uuid.Parse(chi.URLParam(r, "modelId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid model id")
		return
	}
	var req patchPlatformAIModelReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := h.llmRepo.SetModelEnabled(r.Context(), modelID, req.Enabled); err != nil {
		if errors.Is(err, llm.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "model not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type patchPlatformAIModelOrderReq struct {
	ModelIDs []string `json:"modelIds"`
}

// PatchPlatformAIModelOrder godoc
//
//	@Summary		Reorder a provider's models
//	@Description	Sets the try-order for provider's models — modelIds[0] is tried first by the waterfall, modelIds[1] is the fallback, and so on.
//	@Tags			AI Provider Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			provider	path	string							true	"groq | gemini | claude | openai"
//	@Param			body		body	patchPlatformAIModelOrderReq	true	"Model IDs in the desired try-order"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/{provider}/order [patch]
func (h *Handler) PatchPlatformAIModelOrder(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !isKnownChatProvider(provider) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_provider", "unsupported provider")
		return
	}
	var req patchPlatformAIModelOrderReq
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
	if err := h.llmRepo.SetModelOrder(r.Context(), llm.ProviderName(provider), ids); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DeletePlatformAIModel godoc
//
//	@Summary		Remove a model the superadmin added
//	@Tags			AI Provider Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			modelId	path	string	true	"Model ID"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Router			/admin/ai-providers/models/{modelId} [delete]
func (h *Handler) DeletePlatformAIModel(w http.ResponseWriter, r *http.Request) {
	modelID, err := uuid.Parse(chi.URLParam(r, "modelId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid model id")
		return
	}
	if err := h.llmRepo.DeleteModel(r.Context(), modelID); err != nil {
		if errors.Is(err, llm.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "model not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── Mistral (OCR-only, per-studio BYOK — unaffected by AI provider centralization) ──

// GetStudioMistralKey godoc
//
//	@Summary		Get a studio's Mistral OCR key status
//	@Description	Presence/suffix only — the actual key value is never returned.
//	@Tags			AI Assistant Settings
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string	true	"Studio ID"
//	@Success		200			{object}	map[string]any
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/me/studios/{studioId}/mistral-key [get]
func (h *Handler) GetStudioMistralKey(w http.ResponseWriter, r *http.Request) {
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
	httpx.JSON(w, http.StatusOK, map[string]any{
		"hasApiKey":      studio.MistralAPIKey != "",
		"keyFingerprint": studio.MistralAPIKeyFingerprint,
	})
}

type putStudioMistralKeyReq struct {
	APIKey string `json:"apiKey"`
}

// PutStudioMistralKey godoc
//
//	@Summary		Save a studio's Mistral OCR API key
//	@Description	Mistral is used for Knowledge Base image OCR only, not the chat-reply waterfall — it stays a per-studio key.
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string					true	"Studio ID"
//	@Param			body		body	putStudioMistralKeyReq	true	"API key"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/me/studios/{studioId}/mistral-key [put]
func (h *Handler) PutStudioMistralKey(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req putStudioMistralKeyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.svc.repo.UpdateAIProviderKey(r.Context(), studioID, "mistral", req.APIKey, &c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	h.logSecurityEvent(r.Context(), r, &c.UserID, string(c.Role), &studioID, "studio_secret.set", secaudit.OutcomeSuccess, map[string]any{"provider": "mistral"})
	if riskalert.SecretChanged() {
		h.sendRiskAlert("Studio secret changed", fmt.Sprintf("Studio %s set/replaced its mistral key.", studioID))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PostStudioMistralTestKey godoc
//
//	@Summary		Test a studio's Mistral OCR API key
//	@Tags			AI Assistant Settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			studioId	path	string					true	"Studio ID"
//	@Param			body		body	putStudioMistralKeyReq	true	"API key (omit to test the already-saved key)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	httpx.ErrorResponse
//	@Failure		422			{object}	httpx.ErrorResponse	"the live test failed"
//	@Router			/me/studios/{studioId}/mistral-key/test [post]
func (h *Handler) PostStudioMistralTestKey(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}
	var req putStudioMistralKeyReq
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
		apiKey = studio.MistralAPIKey
	}
	if apiKey == "" {
		httpx.WriteValidationError(w, map[string]string{"apiKey": "required"})
		return
	}

	testCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if testErr := mistral.New().ValidateKey(testCtx, apiKey); testErr != nil {
		httpx.WriteValidationError(w, map[string]string{"apiKey": testErr.Error()})
		return
	}

	if strings.TrimSpace(req.APIKey) != "" {
		c := identity.MustClaims(r.Context())
		if err := h.svc.repo.UpdateAIProviderKey(r.Context(), studioID, "mistral", req.APIKey, &c.UserID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
