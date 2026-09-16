package decisiontree

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

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

	r.Get("/decision-trees/import-template", h.importTemplate)
	r.Post("/decision-trees/{treeId}/nodes/import", h.importNodes)
	r.Get("/decision-trees/{treeId}/export", h.exportTree)

	r.Post("/decision-trees/suggest-keywords", h.suggestKeywords)
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

// createTree godoc
//
//	@Summary		Create a decision tree
//	@Description	Creates a new decision tree for the studio, with a name and the lead statuses it applies to.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string			true	"Studio ID (UUID)"
//	@Param			body		body		createTreeReq	true	"Tree name and target lead statuses"
//	@Success		201			{object}	Tree
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		422			{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees [post]
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

// listTrees godoc
//
//	@Summary		List decision trees
//	@Description	Returns all decision trees belonging to the studio.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees [get]
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

// getTree godoc
//
//	@Summary		Get a decision tree
//	@Description	Returns a single decision tree, including its nodes.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			treeId		path		string	true	"Decision tree ID"
//	@Success		200			{object}	Tree
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or treeId"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId} [get]
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

// updateTree godoc
//
//	@Summary		Update a decision tree
//	@Description	Partially updates a decision tree's name and/or active flag. If "targetStatuses" is present in the payload its full list is replaced; if absent, target statuses are left unchanged.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string					true	"Studio ID (UUID)"
//	@Param			treeId		path		string					true	"Decision tree ID"
//	@Param			body		body		map[string]interface{}	true	"Fields to update: name, isActive, targetStatuses"
//	@Success		200			{object}	Tree
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId, treeId, or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId} [patch]
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

// deleteTree godoc
//
//	@Summary		Delete a decision tree
//	@Description	Deletes a decision tree and its nodes.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			treeId		path		string	true	"Decision tree ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or treeId"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId} [delete]
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
	PositionX      *float64       `json:"positionX"`
	PositionY      *float64       `json:"positionY"`
}

// createNode godoc
//
//	@Summary		Create a decision tree node
//	@Description	Adds a node to a decision tree, optionally nested under a parent node.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string			true	"Studio ID (UUID)"
//	@Param			treeId		path		string			true	"Decision tree ID"
//	@Param			body		body		createNodeReq	true	"Node payload"
//	@Success		201			{object}	Node
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId, treeId, or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		422			{object}	httpx.ErrorResponse	"validation failed"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/nodes [post]
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
		PositionX:      req.PositionX,
		PositionY:      req.PositionY,
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
	PositionX      *float64       `json:"positionX"`
	PositionY      *float64       `json:"positionY"`
}

// updateNode godoc
//
//	@Summary		Update a decision tree node
//	@Description	Partially updates a decision tree node's fields (label, condition, reply template, action, position, sort order, etc.).
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string			true	"Studio ID (UUID)"
//	@Param			treeId		path		string			true	"Decision tree ID"
//	@Param			nodeId		path		string			true	"Node ID"
//	@Param			body		body		updateNodeReq	true	"Fields to update"
//	@Success		200			{object}	Node
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId, treeId, nodeId, or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree or node not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/nodes/{nodeId} [patch]
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
		PositionX:      req.PositionX,
		PositionY:      req.PositionY,
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

type suggestKeywordsReq struct {
	Label         string `json:"label"`
	ReplyTemplate string `json:"replyTemplate"`
}

// suggestKeywords godoc
//
//	@Summary		Suggest keywords for a decision node
//	@Description	Uses the studio's AI provider to suggest keyword phrases for a node given its label and reply template text.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string				true	"Studio ID (UUID)"
//	@Param			body		body		suggestKeywordsReq	true	"Node label and reply template to derive keywords from"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		422			{object}	httpx.ErrorResponse	"label is required"
//	@Failure		502			{object}	httpx.ErrorResponse	"keyword suggestion failed"
//	@Router			/api/v1/studios/{studioId}/decision-trees/suggest-keywords [post]
func (h *Handler) suggestKeywords(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	var req suggestKeywordsReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		httpx.WriteValidationError(w, map[string]string{"label": "required"})
		return
	}
	keywords, err := h.svc.SuggestKeywords(r.Context(), studioID, req.Label, req.ReplyTemplate)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "suggest_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"keywords": keywords})
}

// deleteNode godoc
//
//	@Summary		Delete a decision tree node
//	@Description	Deletes a single node from a decision tree.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			treeId		path		string	true	"Decision tree ID"
//	@Param			nodeId		path		string	true	"Node ID"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId, treeId, or nodeId"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree or node not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/nodes/{nodeId} [delete]
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

// simulate godoc
//
//	@Summary		Simulate a message against a decision tree
//	@Description	Walks the decision tree with the given message (and optional lead status override) and returns which node/reply/action would trigger, without affecting any real lead.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			json
//	@Produce		json
//	@Param			studioId	path		string		true	"Studio ID (UUID)"
//	@Param			treeId		path		string		true	"Decision tree ID"
//	@Param			body		body		simulateReq	true	"Message to test, and optional lead status"
//	@Success		200			{object}	SimulateResult
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId, treeId, or malformed body"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		422			{object}	httpx.ErrorResponse	"message is required"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/simulate [post]
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

// ----- bulk import -----

var importTemplateHeaders = []string{
	"Label", "Parent Label", "Condition Type", "Condition Value",
	"Reply Template", "Action", "Action Value", "Sort Order",
}

// importTemplate serves a starter .xlsx a studio owner can fill in and
// re-upload via importNodes. Not studio-scoped — it's a static file, not data.
//
// importTemplate godoc
//
//	@Summary		Download the node import template
//	@Description	Returns a starter .xlsx workbook (with headers and example rows) a studio owner can fill in and re-upload via the node import endpoint. Not studio-data-specific — it's a static file and does not read from the database.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Success		200			{file}	binary	"decision-tree-template.xlsx"
//	@Failure		500			{object}	httpx.ErrorResponse	"failed to generate template"
//	@Router			/api/v1/studios/{studioId}/decision-trees/import-template [get]
func (h *Handler) importTemplate(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)

	for i, header := range importTemplateHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, header)
	}

	example := [][]any{
		{"Ask about pricing", "", "keyword", "price,cost,how much", "", "reply", "", 1},
		{"Share pricing", "Ask about pricing", "default", "", "Our trial is $49 and monthly plans start at $129. Want to book a trial?", "reply", "", 1},
		{"Ready to book", "Ask about pricing", "keyword", "book,trial,sign up", "", "book_trial", "", 2},
		{"Complaint", "", "intent", "complaint", "", "escalate_human", "", 2},
		{"Became a member", "", "keyword", "i joined,i'm a member", "", "change_status", "member", 3},
	}
	for r, row := range example {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	f.SetColWidth(sheet, "A", "B", 24)
	f.SetColWidth(sheet, "C", "D", 20)
	f.SetColWidth(sheet, "E", "E", 40)
	f.SetColWidth(sheet, "F", "H", 16)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="decision-tree-template.xlsx"`)
	if err := f.Write(w); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to generate template")
		return
	}
}

// exportTree downloads a whole tree's nodes as an .xlsx in the exact same
// column layout importTemplate/importNodes use, so it can be re-uploaded via
// "Import" on a different studio's (empty) tree to recreate the same flow.
//
// exportTree godoc
//
//	@Summary		Export a decision tree
//	@Description	Downloads a whole tree's nodes as an .xlsx file in the same column layout the import template/import endpoint use, so it can be re-uploaded to recreate the same flow on another studio's tree.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Produce		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	@Param			studioId	path	string	true	"Studio ID (UUID)"
//	@Param			treeId		path	string	true	"Decision tree ID"
//	@Success		200			{file}	binary	"<tree name>.xlsx"
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId or treeId"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		500			{object}	httpx.ErrorResponse	"failed to generate export"
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/export [get]
func (h *Handler) exportTree(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}
	tree, err := h.svc.GetTree(r.Context(), studioID, treeID)
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)

	for i, header := range importTemplateHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, header)
	}

	rowNum := 2
	var writeNode func(n Node, parentLabel string)
	writeNode = func(n Node, parentLabel string) {
		row := []any{
			n.Label,
			parentLabel,
			string(n.ConditionType),
			conditionValueToCell(n.ConditionType, n.ConditionValue),
			n.ReplyTemplate,
			string(n.Action),
			actionValueToCell(n.Action, n.ActionValue),
			n.SortOrder,
		}
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			f.SetCellValue(sheet, cell, val)
		}
		rowNum++
		for _, child := range n.Children {
			writeNode(child, n.Label)
		}
	}
	for _, n := range tree.Nodes {
		writeNode(n, "")
	}

	f.SetColWidth(sheet, "A", "B", 24)
	f.SetColWidth(sheet, "C", "D", 20)
	f.SetColWidth(sheet, "E", "E", 40)
	f.SetColWidth(sheet, "F", "H", 16)

	filename := strings.ReplaceAll(tree.Name, `"`, "")
	if filename == "" {
		filename = "decision-tree"
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, filename))
	if err := f.Write(w); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to generate export")
		return
	}
}

// toStringSlice reads a ConditionValue field back out regardless of whether
// it decoded as []string (freshly created in-process) or []interface{} (the
// common shape after a JSONB round-trip through the database).
func toStringSlice(v any) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// conditionValueToCell is the inverse of conditionValueToJSON — renders a
// node's ConditionValue map back into the plain-text cell format the import
// template expects for that condition type.
func conditionValueToCell(ct ConditionType, cv ConditionValue) string {
	if cv == nil {
		return ""
	}
	switch ct {
	case ConditionKeyword:
		return strings.Join(toStringSlice(cv["keywords"]), ",")
	case ConditionIntent:
		if s, ok := cv["intent"].(string); ok {
			return s
		}
		return ""
	case ConditionSentiment:
		if s, ok := cv["sentiment"].(string); ok {
			return s
		}
		return ""
	case ConditionLeadStatus:
		return strings.Join(toStringSlice(cv["statuses"]), ",")
	default:
		return ""
	}
}

// actionValueToCell is the inverse of actionValueToJSON.
func actionValueToCell(a Action, av ConditionValue) string {
	if a == ActionChangeStatus && av != nil {
		if s, ok := av["target_status"].(string); ok {
			return s
		}
	}
	return ""
}

// importNodes bulk-creates nodes for an existing tree from an uploaded
// .xlsx/.csv file matching the importTemplate column layout. Rows are
// resolved parent-first regardless of sheet order (see Service.ImportNodes).
// importNodes godoc
//
//	@Summary		Bulk import decision tree nodes
//	@Description	Uploads an .xlsx file matching the import template columns and bulk-creates nodes on an existing tree. Rows are resolved parent-first regardless of sheet order. The response always includes a full (possibly empty) list of per-row errors.
//	@Tags			Decision Trees
//	@Security		CookieAuth
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID (UUID)"
//	@Param			treeId		path		string	true	"Decision tree ID"
//	@Param			file		formData	file	true	"Excel file (.xlsx) matching the import template column layout"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"invalid studioId/treeId, missing file, malformed multipart form, or unreadable Excel file"
//	@Failure		403			{object}	httpx.ErrorResponse	"no studio bound to this user"
//	@Failure		404			{object}	httpx.ErrorResponse	"tree not found"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Router			/api/v1/studios/{studioId}/decision-trees/{treeId}/nodes/import [post]
func (h *Handler) importNodes(w http.ResponseWriter, r *http.Request) {
	studioID, ok := h.resolveStudioID(w, r)
	if !ok {
		return
	}
	treeID, err := uuid.Parse(chi.URLParam(r, "treeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid treeId")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "failed to parse multipart form")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file field is required")
		return
	}
	defer file.Close()

	xf, err := excelize.OpenReader(file)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to open Excel file: %v", err))
		return
	}
	defer xf.Close()
	sheet := xf.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	sheetRows, err := xf.GetRows(sheet)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_excel", fmt.Sprintf("failed to read Excel sheet: %v", err))
		return
	}
	if len(sheetRows) < 2 {
		httpx.WriteError(w, http.StatusBadRequest, "empty_file", "file has no data rows below the header")
		return
	}

	rows, parseErrs := parseImportRows(sheetRows)
	created, importErrs, err := h.svc.ImportNodes(r.Context(), studioID, treeID, rows)
	if errors.Is(err, ErrTreeNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "tree not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	// rowErrors must always serialize as a JSON array, never null — a nil
	// Go slice marshals to `null`, which crashes frontend code that calls
	// .length on the "errors" field after a fully successful import.
	rowErrors := append(parseErrs, importErrs...)
	if rowErrors == nil {
		rowErrors = []ImportRowError{}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"created": created,
		"errors":  rowErrors,
	})
}

// parseImportRows converts raw spreadsheet cells (as read by excelize) into
// ImportRow values, collecting per-row parse errors (bad Condition
// Type/Action values) separately from rows that parsed fine.
func parseImportRows(sheetRows [][]string) ([]ImportRow, []ImportRowError) {
	var rows []ImportRow
	var errs []ImportRowError

	for i, raw := range sheetRows {
		if i == 0 {
			continue // header
		}
		rowNum := i + 1 // 1-based, matches what a spreadsheet app shows
		get := func(col int) string {
			if col >= len(raw) {
				return ""
			}
			return strings.TrimSpace(raw[col])
		}
		label := get(0)
		parentLabel := get(1)
		conditionType := ConditionType(strings.ToLower(get(2)))
		conditionValueRaw := get(3)
		replyTemplate := get(4)
		action := Action(strings.ToLower(get(5)))
		actionValueRaw := get(6)
		sortOrderRaw := get(7)

		if label == "" && parentLabel == "" && conditionValueRaw == "" && replyTemplate == "" {
			continue // blank row
		}
		if label == "" {
			errs = append(errs, ImportRowError{RowNum: rowNum, Error: "Label is required"})
			continue
		}

		sortOrder := 0
		if sortOrderRaw != "" {
			if n, err := strconv.Atoi(sortOrderRaw); err == nil {
				sortOrder = n
			}
		}

		row := ImportRow{
			RowNum:         rowNum,
			Label:          label,
			ParentLabel:    parentLabel,
			ConditionType:  conditionType,
			ReplyTemplate:  replyTemplate,
			Action:         action,
			ConditionValue: conditionValueToJSON(conditionType, conditionValueRaw),
			ActionValue:    actionValueToJSON(action, actionValueRaw),
			SortOrder:      sortOrder,
		}
		rows = append(rows, row)
	}
	return rows, errs
}

// conditionValueToJSON converts the plain-text "Condition Value" cell into
// the ConditionValue map shape CreateNode expects, per condition type.
func conditionValueToJSON(ct ConditionType, raw string) ConditionValue {
	switch ct {
	case ConditionKeyword:
		if raw == "" {
			return ConditionValue{"keywords": []string{}}
		}
		parts := strings.Split(raw, ",")
		keywords := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				keywords = append(keywords, p)
			}
		}
		return ConditionValue{"keywords": keywords}
	case ConditionIntent:
		return ConditionValue{"intent": strings.TrimSpace(raw)}
	case ConditionSentiment:
		return ConditionValue{"sentiment": strings.TrimSpace(raw)}
	case ConditionLeadStatus:
		parts := strings.Split(raw, ",")
		statuses := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				statuses = append(statuses, p)
			}
		}
		return ConditionValue{"statuses": statuses}
	default:
		return ConditionValue{}
	}
}

// actionValueToJSON converts the plain-text "Action Value" cell — only
// meaningful for change_status, where it's the target lead status.
func actionValueToJSON(a Action, raw string) ConditionValue {
	if a == ActionChangeStatus && raw != "" {
		return ConditionValue{"target_status": strings.TrimSpace(raw)}
	}
	return ConditionValue{}
}
