package crm

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	repo     *Repo
	executor *Executor
	llmRepo  *llm.Repo
	resolver *llm.Resolver
}

func NewHandler(repo *Repo, executor *Executor, llmRepo *llm.Repo, resolver *llm.Resolver) *Handler {
	return &Handler{repo: repo, executor: executor, llmRepo: llmRepo, resolver: resolver}
}

// AdminRoutes are mounted under /api/v1/admin, already wrapped in
// identity.RequireRole(identity.RoleSuperAdmin) by cmd/server/main.go — same
// gating as studiosHandler.AdminRoutes / leadsHandler.AdminRoutes.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/crm-providers", h.listProviders)
	r.Post("/crm-providers", h.createProvider)
	r.Post("/crm-providers/parse", h.parseProvider)
	r.Put("/crm-providers/{id}", h.updateProvider)
	r.Get("/crm-providers/{id}/operations", h.listOperations)
	r.Put("/crm-providers/{id}/operations/{opId}", h.updateOperation)
	r.Post("/crm-providers/{id}/activate", h.activateProvider)

	r.Get("/studios/{studioId}/crm-connections", h.getConnection)
	r.Post("/studios/{studioId}/crm-connections", h.createConnection)
	r.Delete("/studios/{studioId}/crm-connections/{providerId}", h.deleteConnection)

	r.Get("/ai-task-configs/{purpose}", h.getAITaskConfig)
	r.Put("/ai-task-configs/{purpose}", h.updateAITaskConfig)
}

// ============================================================
// providers
// ============================================================

func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListProviders(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"providers": list})
}

type createProviderReq struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	BaseURL       string         `json:"baseUrl"`
	AuthType      AuthType       `json:"authType"`
	AuthFieldDefs []AuthFieldDef `json:"authFieldDefs"`
	SpecSource    string         `json:"specSource"`
}

// createProvider creates a draft provider directly (hand-defined operations,
// e.g. seeding Glofox). The AI-assisted "upload a doc, get a proposed
// mapping" flow is a separate endpoint added in Phase 2.
func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	var req createProviderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httpx.WriteValidationError(w, map[string]string{"name": "required"})
		return
	}
	c := identity.MustClaims(r.Context())
	p := &Provider{
		Name:          req.Name,
		Description:   req.Description,
		BaseURL:       req.BaseURL,
		AuthType:      req.AuthType,
		AuthFieldDefs: req.AuthFieldDefs,
		SpecSource:    req.SpecSource,
		Status:        ProviderDraft,
		CreatedBy:     &c.UserID,
	}
	if err := h.repo.CreateProvider(r.Context(), p); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

type updateProviderReq struct {
	Description           string            `json:"description"`
	BaseURL               string            `json:"baseUrl"`
	AuthType              AuthType          `json:"authType"`
	AuthFieldDefs         []AuthFieldDef    `json:"authFieldDefs"`
	TokenLoginPath        string            `json:"tokenLoginPath"`
	TokenLoginMethod      string            `json:"tokenLoginMethod"`
	TokenLoginBodyMapping map[string]string `json:"tokenLoginBodyMapping"`
	TokenResponsePath     string            `json:"tokenResponsePath"`
	TokenExpiryPath       string            `json:"tokenExpiryPath"`
	TokenExpirySeconds    int               `json:"tokenExpirySeconds"`
}

// updateProvider is how a super-admin corrects the AI's guess at base URL /
// auth scheme before activating a draft provider.
func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid provider id")
		return
	}
	var req updateProviderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	in := UpdateProviderInput{
		Description:           req.Description,
		BaseURL:               req.BaseURL,
		AuthType:              req.AuthType,
		AuthFieldDefs:         req.AuthFieldDefs,
		TokenLoginPath:        req.TokenLoginPath,
		TokenLoginMethod:      req.TokenLoginMethod,
		TokenLoginBodyMapping: req.TokenLoginBodyMapping,
		TokenResponsePath:     req.TokenResponsePath,
		TokenExpiryPath:       req.TokenExpiryPath,
		TokenExpirySeconds:    req.TokenExpirySeconds,
	}
	if err := h.repo.UpdateProvider(r.Context(), id, in); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	p, err := h.repo.GetProvider(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) activateProvider(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid provider id")
		return
	}
	if err := h.repo.ActivateProvider(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type parseProviderReq struct {
	Name    string `json:"name"`
	DocText string `json:"docText"`
}

type parseProviderRes struct {
	Provider   *Provider   `json:"provider"`
	Operations []Operation `json:"operations"`
}

// parseProvider is the AI-assisted onboarding step: an uploaded/pasted CRM
// doc goes to whichever LLM is configured for crm.ParsePurpose, comes back
// as a draft provider + operations for a super-admin to review before
// activateProvider makes it connectable.
func (h *Handler) parseProvider(w http.ResponseWriter, r *http.Request) {
	var req parseProviderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.DocText == "" {
		httpx.WriteValidationError(w, map[string]string{"name": "required", "docText": "required"})
		return
	}
	provider, err := h.resolver.ResolveProvider(r.Context(), ParsePurpose)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "llm_not_configured", err.Error())
		return
	}
	p, ops, err := ParseCRMDoc(r.Context(), provider, h.repo, req.Name, req.DocText)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "parse_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, parseProviderRes{Provider: p, Operations: ops})
}

// ============================================================
// operations
// ============================================================

func (h *Handler) listOperations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid provider id")
		return
	}
	list, err := h.repo.ListOperations(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"operations": list})
}

type updateOperationReq struct {
	OperationKey    OperationKey   `json:"operationKey"`
	HTTPMethod      string         `json:"httpMethod"`
	PathTemplate    string         `json:"pathTemplate"`
	RequestMapping  map[string]any `json:"requestMapping"`
	ResponseMapping map[string]any `json:"responseMapping"`
	Reviewed        bool           `json:"reviewed"`
}

// updateOperation is how a super-admin edits an AI-proposed (or hand-typed)
// mapping — {id} is the provider, {opId} is unused beyond routing clarity
// since operation_key is the real identity within a provider; the body's
// operationKey is what's actually upserted.
func (h *Handler) updateOperation(w http.ResponseWriter, r *http.Request) {
	providerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid provider id")
		return
	}
	var req updateOperationReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !req.OperationKey.Valid() {
		httpx.WriteValidationError(w, map[string]string{"operationKey": "must be one of the fixed catalog keys"})
		return
	}
	op := &Operation{
		CRMProviderID:   providerID,
		OperationKey:    req.OperationKey,
		HTTPMethod:      req.HTTPMethod,
		PathTemplate:    req.PathTemplate,
		RequestMapping:  req.RequestMapping,
		ResponseMapping: req.ResponseMapping,
		Reviewed:        req.Reviewed,
	}
	if err := h.repo.CreateOperation(r.Context(), op); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, op)
}

// ============================================================
// studio connections
// ============================================================

// getConnection reports the studio's one active CRM connection (v1 supports
// only one at a time — see GetActiveConnectionForStudio), without ever
// exposing decrypted credentials, so the frontend can show "connected to
// Glofox" / "not connected" before offering to connect or disconnect.
func (h *Handler) getConnection(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	conn, err := h.repo.GetActiveConnectionForStudio(r.Context(), studioID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.JSON(w, http.StatusOK, map[string]any{"connected": false})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"connected": true, "connection": conn})
}

type createConnectionReq struct {
	CRMProviderID uuid.UUID         `json:"crmProviderId"`
	Credentials   map[string]string `json:"credentials"`
}

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	var req createConnectionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.CRMProviderID == uuid.Nil {
		httpx.WriteValidationError(w, map[string]string{"crmProviderId": "required"})
		return
	}
	conn, err := h.repo.CreateConnection(r.Context(), studioID, req.CRMProviderID, req.Credentials)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, conn)
}

func (h *Handler) deleteConnection(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid studio id")
		return
	}
	providerID, err := uuid.Parse(chi.URLParam(r, "providerId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid provider id")
		return
	}
	if err := h.repo.DisconnectConnection(r.Context(), studioID, providerID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "connection not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ============================================================
// ai_task_configs — which LLM handles which AI-driven admin task.
// Not CRM-specific; lives here for now since crm_doc_parsing is the only
// task using it, but the shape (GET/PUT by purpose) is generic.
// ============================================================

func (h *Handler) getAITaskConfig(w http.ResponseWriter, r *http.Request) {
	purpose := chi.URLParam(r, "purpose")
	cfg, err := h.llmRepo.GetTaskConfig(r.Context(), purpose)
	if err != nil {
		if errors.Is(err, llm.ErrNotFound) {
			httpx.JSON(w, http.StatusOK, map[string]any{"purpose": purpose, "configured": false})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"purpose":    cfg.Purpose,
		"provider":   cfg.Provider,
		"model":      cfg.Model,
		"hasApiKey":  cfg.APIKeyEnc != "",
		"configured": true,
	})
}

type updateAITaskConfigReq struct {
	Provider llm.ProviderName `json:"provider"`
	Model    string           `json:"model"`
	APIKey   string           `json:"apiKey"` // "" leaves any existing override untouched
}

func (h *Handler) updateAITaskConfig(w http.ResponseWriter, r *http.Request) {
	purpose := chi.URLParam(r, "purpose")
	var req updateAITaskConfigReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" || req.Model == "" {
		httpx.WriteValidationError(w, map[string]string{"provider": "required", "model": "required"})
		return
	}
	c := identity.MustClaims(r.Context())
	if err := h.llmRepo.UpsertTaskConfig(r.Context(), purpose, req.Provider, req.Model, req.APIKey, c.UserID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
