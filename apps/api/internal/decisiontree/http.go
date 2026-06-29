package decisiontree

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/identity"
	"github.com/projectx/api/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// AdminRoutes mounts under /studios/{studioId}
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/decision-trees", h.listTrees)
	r.Post("/decision-trees", h.createTree)
	r.Get("/decision-trees/{treeId}", h.getTree)
	r.Patch("/decision-trees/{treeId}", h.updateTree)
	r.Delete("/decision-trees/{treeId}", h.deleteTree)

	r.Post("/decision-trees/{treeId}/nodes", h.createNode)
	r.Patch("/decision-trees/{treeId}/nodes/{nodeId}", h.updateNode)
	r.Delete("/decision-trees/{treeId}/nodes/{nodeId}", h.deleteNode)

	r.Post("/decision-trees/{treeId}/simulate", h.simulate)
}

func (h *Handler) resolveStudioID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	c := identity.MustClaims(r.Context())
	if c.IsSuper() {
		studioIDStr := chi.URLParam(r, "studioId")
		if studioIDStr == "" {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "studioId parameter required")
			return uuid.Nil, false
		}
		studioID, err := uuid.Parse(studioIDStr)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid studioId")
			return uuid.Nil, false
		}
		return studioID, true
	}
	if c.StudioID == nil {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "no studio bound to this user")
		return uuid.Nil, false
	}
	return *c.StudioID, true
}

// ----- trees -----

type createTreeReq struct {
	Name           string   `json:"name"`
	TargetStatuses []string `json:"targetStatuses"`
}

func (h *Handler) createTree(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	var req createTreeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	t, errs, err := h.svc.CreateTree(r.Context(), studioID, CreateTreeInput{
		Name:           req.Name,
		TargetStatuses: req.TargetStatuses,
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) listTrees(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	trees, err := h.svc.ListTrees(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"trees": trees})
}

func (h *Handler) getTree(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	t, err := h.svc.GetTree(r.Context(), studioID, treeID)
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

type updateTreeReq struct {
	Name           *string  `json:"name"`
	IsActive       *bool    `json:"isActive"`
	TargetStatuses []string `json:"targetStatuses"` // present in payload = update; absent = leave unchanged
}

func (h *Handler) updateTree(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	// Use raw JSON decode so we can detect whether targetStatuses was sent.
	var raw map[string]any
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}
	inp := UpdateTreeInput{}
	if v, ok := raw["name"].(string); ok {
		inp.Name = &v
	}
	if v, ok := raw["isActive"].(bool); ok {
		inp.IsActive = &v
	}
	if _, ok := raw["targetStatuses"]; ok {
		inp.UpdateStatuses = true
		if arr, ok := raw["targetStatuses"].([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					inp.TargetStatuses = append(inp.TargetStatuses, s)
				}
			}
		} else {
			inp.TargetStatuses = []string{}
		}
	}
	t, err := h.svc.UpdateTree(r.Context(), studioID, treeID, inp)
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) deleteTree(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	if err := h.svc.DeleteTree(r.Context(), studioID, treeID); err != nil {
		if errors.Is(err, ErrTreeNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ----- nodes -----

type createNodeReq struct {
	ParentID       *uuid.UUID     `json:"parentId"`
	Label          string         `json:"label"`
	ConditionType  ConditionType  `json:"conditionType"`
	ConditionValue ConditionValue `json:"conditionValue"`
	ReplyTemplate  string         `json:"replyTemplate"`
	Action         Action         `json:"action"`
	ActionValue    ConditionValue `json:"actionValue"`
	SortOrder      int            `json:"sortOrder"`
}

func (h *Handler) createNode(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	var req createNodeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	n, errs, err := h.svc.CreateNode(r.Context(), studioID, treeID, CreateNodeInput{
		ParentID:       req.ParentID,
		Label:          req.Label,
		ConditionType:  req.ConditionType,
		ConditionValue: req.ConditionValue,
		ReplyTemplate:  req.ReplyTemplate,
		Action:         req.Action,
		ActionValue:    req.ActionValue,
		SortOrder:      req.SortOrder,
	})
	if errs != nil {
		httpx.WriteValidationError(w, errs)
		return
	}
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusCreated, n)
}

type updateNodeReq struct {
	Label          *string        `json:"label"`
	ConditionType  *ConditionType `json:"conditionType"`
	ConditionValue ConditionValue `json:"conditionValue"`
	ReplyTemplate  *string        `json:"replyTemplate"`
	Action         *Action        `json:"action"`
	ActionValue    ConditionValue `json:"actionValue"`
	SortOrder      *int           `json:"sortOrder"`
}

func (h *Handler) updateNode(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	nodeID, err := uuid.Parse(chi.URLParam(r, "nodeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid nodeId")
		return
	}
	var req updateNodeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	n, err := h.svc.UpdateNode(r.Context(), studioID, treeID, nodeID, UpdateNodeInput{
		Label:          req.Label,
		ConditionType:  req.ConditionType,
		ConditionValue: req.ConditionValue,
		ReplyTemplate:  req.ReplyTemplate,
		Action:         req.Action,
		ActionValue:    req.ActionValue,
		SortOrder:      req.SortOrder,
	})
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if errors.Is(err, ErrNodeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, n)
}

func (h *Handler) deleteNode(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	nodeID, err := uuid.Parse(chi.URLParam(r, "nodeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid nodeId")
		return
	}
	if err := h.svc.DeleteNode(r.Context(), studioID, treeID, nodeID); err != nil {
		if errors.Is(err, ErrTreeNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
			return
		}
		if errors.Is(err, ErrNodeNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ----- simulate -----

type simulateReq struct {
	Message    string `json:"message"`
	LeadStatus string `json:"leadStatus"` // optional: test as if lead has this status
}

func (h *Handler) simulate(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	var req simulateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Message == "" {
		httpx.WriteValidationError(w, map[string]string{"message": "required"})
		return
	}
	result, err := h.svc.Simulate(r.Context(), studioID, treeID, req.Message, req.LeadStatus)
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
