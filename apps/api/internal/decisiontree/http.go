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
	PositionX      *float64       `json:"positionX"`
	PositionY      *float64       `json:"positionY"`
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

// ----- bulk import -----

var importTemplateHeaders = []string{
	"Label", "Parent Label", "Condition Type", "Condition Value",
	"Reply Template", "Action", "Action Value", "Sort Order",
}

// importTemplate serves a starter .xlsx a studio owner can fill in and
// re-upload via importNodes. Not studio-scoped — it's a static file, not data.
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

// importNodes bulk-creates nodes for an existing tree from an uploaded
// .xlsx/.csv file matching the importTemplate column layout. Rows are
// resolved parent-first regardless of sheet order (see Service.ImportNodes).
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
