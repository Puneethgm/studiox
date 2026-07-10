package decisiontree

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

func (s *Service) CreateTree(ctx context.Context, studioID uuid.UUID, input CreateTreeInput) (*Tree, map[string]string, error) {
	if errs := validateTree(input); errs != nil {
		return nil, errs, nil
	}
	t, err := s.repo.CreateTree(ctx, studioID, input)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return t, nil, err
}

func (s *Service) ListTrees(ctx context.Context, studioID uuid.UUID) ([]Tree, error) {
	return s.repo.ListTrees(ctx, studioID)
}

func (s *Service) GetTree(ctx context.Context, studioID, treeID uuid.UUID) (*Tree, error) {
	return s.repo.GetTree(ctx, studioID, treeID)
}

func (s *Service) UpdateTree(ctx context.Context, studioID, treeID uuid.UUID, input UpdateTreeInput) (*Tree, error) {
	if input.IsActive != nil && *input.IsActive {
		// Deactivate any other active tree that targets the same statuses.
		// - Activating a catch-all tree (empty statuses) deactivates other catch-all trees.
		// - Activating a status-specific tree deactivates others that share any of those statuses.
		// This lets multiple trees be active simultaneously as long as they target distinct groups.
		trees, err := s.repo.ListTrees(ctx, studioID)
		if err != nil {
			return nil, err
		}
		f := false
		incomingStatuses := input.TargetStatuses
		if !input.UpdateStatuses {
			// Fetch current statuses of the tree being activated so we can compare correctly.
			if cur, err := s.repo.GetTree(ctx, studioID, treeID); err == nil {
				incomingStatuses = cur.TargetStatuses
			}
		}
		for _, t := range trees {
			if t.ID == treeID || !t.IsActive {
				continue
			}
			if overlaps(incomingStatuses, t.TargetStatuses) {
				if _, err := s.repo.UpdateTree(ctx, studioID, t.ID, UpdateTreeInput{IsActive: &f}); err != nil {
					return nil, err
				}
			}
		}
	}
	t, err := s.repo.UpdateTree(ctx, studioID, treeID, input)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return t, err
}

// overlaps returns true if two status lists share any element, or both are empty (both catch-all).
func overlaps(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true // both catch-all
	}
	for _, x := range a {
		for _, y := range b {
			if strings.EqualFold(x, y) {
				return true
			}
		}
	}
	return false
}

func (s *Service) DeleteTree(ctx context.Context, studioID, treeID uuid.UUID) error {
	err := s.repo.DeleteTree(ctx, studioID, treeID)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return err
}

func (s *Service) CreateNode(ctx context.Context, studioID, treeID uuid.UUID, input CreateNodeInput) (*Node, map[string]string, error) {
	// Verify tree belongs to studio.
	if _, err := s.repo.GetTree(ctx, studioID, treeID); err != nil {
		return nil, nil, err
	}
	if errs := validateNode(input); errs != nil {
		return nil, errs, nil
	}
	input.TreeID = treeID
	n, err := s.repo.CreateNode(ctx, input)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return n, nil, err
}

// ImportRow is one parsed spreadsheet row for bulk node import. Label and
// ParentLabel are matched case-insensitively; ParentLabel empty means a root
// node. ConditionValue/ActionValue are plain text as typed in the sheet —
// interpretation depends on ConditionType/Action (see parseImportRow-adjacent
// helpers in http.go).
type ImportRow struct {
	RowNum         int // 1-based spreadsheet row, for error messages
	Label          string
	ParentLabel    string
	ConditionType  ConditionType
	ConditionValue ConditionValue
	ReplyTemplate  string
	Action         Action
	ActionValue    ConditionValue
	SortOrder      int
}

type ImportRowError struct {
	RowNum int    `json:"rowNum"`
	Label  string `json:"label"`
	Error  string `json:"error"`
}

// ImportNodes creates one tree_nodes row per ImportRow, resolving
// ParentLabel -> parent node ID across as many passes as needed so rows can
// appear in the sheet in any order (a child can be listed before its
// parent). Rows whose parent never resolves (typo, or a genuine cycle) are
// reported as errors rather than silently dropped or attached to the root.
func (s *Service) ImportNodes(ctx context.Context, studioID, treeID uuid.UUID, rows []ImportRow) (created int, errs []ImportRowError, err error) {
	tree, err := s.repo.GetTree(ctx, studioID, treeID)
	if err != nil {
		return 0, nil, err
	}

	// labelToID tracks nodes available as parents so far — seeded with
	// every node already in the tree (case-insensitively), then extended as
	// rows in this import succeed, so later rows can reference either an
	// existing node or one created earlier in the same import.
	labelToID := map[string]uuid.UUID{}
	var seedExisting func(nodes []Node)
	seedExisting = func(nodes []Node) {
		for _, n := range nodes {
			labelToID[strings.ToLower(n.Label)] = n.ID
			seedExisting(n.Children)
		}
	}
	seedExisting(tree.Nodes)

	// failedLabels tracks rows in this import that failed validation/creation,
	// so a child whose parent is one of these gets an error that points at
	// the real cause instead of a misleading "not found".
	failedLabels := map[string]bool{}

	remaining := make([]ImportRow, len(rows))
	copy(remaining, rows)

	for len(remaining) > 0 {
		var next []ImportRow
		progressed := false

		for _, row := range remaining {
			var parentID *uuid.UUID
			if row.ParentLabel != "" {
				id, ok := labelToID[strings.ToLower(row.ParentLabel)]
				if !ok {
					// Parent not created yet (maybe later in the sheet) — retry next pass.
					next = append(next, row)
					continue
				}
				parentID = &id
			}

			input := CreateNodeInput{
				TreeID:         treeID,
				ParentID:       parentID,
				Label:          row.Label,
				ConditionType:  row.ConditionType,
				ConditionValue: row.ConditionValue,
				ReplyTemplate:  row.ReplyTemplate,
				Action:         row.Action,
				ActionValue:    row.ActionValue,
				SortOrder:      row.SortOrder,
			}
			if verrs := validateNode(input); verrs != nil {
				msgs := make([]string, 0, len(verrs))
				for field, msg := range verrs {
					msgs = append(msgs, fmt.Sprintf("%s: %s", field, msg))
				}
				errs = append(errs, ImportRowError{RowNum: row.RowNum, Label: row.Label, Error: strings.Join(msgs, "; ")})
				failedLabels[strings.ToLower(row.Label)] = true
				progressed = true
				continue
			}

			n, err := s.repo.CreateNode(ctx, input)
			if err != nil {
				errs = append(errs, ImportRowError{RowNum: row.RowNum, Label: row.Label, Error: err.Error()})
				failedLabels[strings.ToLower(row.Label)] = true
				progressed = true
				continue
			}
			labelToID[strings.ToLower(row.Label)] = n.ID
			created++
			progressed = true
		}

		if !progressed || len(next) == len(remaining) {
			// No row in this pass resolved — every remaining row has an
			// unresolvable Parent Label (typo, a genuine cycle, or the
			// parent row itself failed earlier in this same import).
			for _, row := range next {
				if failedLabels[strings.ToLower(row.ParentLabel)] {
					errs = append(errs, ImportRowError{
						RowNum: row.RowNum, Label: row.Label,
						Error: fmt.Sprintf("parent row %q failed to import — see its error above", row.ParentLabel),
					})
					continue
				}
				errs = append(errs, ImportRowError{
					RowNum: row.RowNum, Label: row.Label,
					Error: fmt.Sprintf("parent label %q not found among rows in this import or the tree", row.ParentLabel),
				})
			}
			break
		}
		remaining = next
	}

	if created > 0 {
		s.repo.InvalidateCache(studioID)
	}
	return created, errs, nil
}

func (s *Service) UpdateNode(ctx context.Context, studioID, treeID, nodeID uuid.UUID, input UpdateNodeInput) (*Node, error) {
	if _, err := s.repo.GetTree(ctx, studioID, treeID); err != nil {
		return nil, err
	}
	n, err := s.repo.UpdateNode(ctx, treeID, nodeID, input)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return n, err
}

func (s *Service) DeleteNode(ctx context.Context, studioID, treeID, nodeID uuid.UUID) error {
	if _, err := s.repo.GetTree(ctx, studioID, treeID); err != nil {
		return err
	}
	err := s.repo.DeleteNode(ctx, treeID, nodeID)
	if err == nil {
		s.repo.InvalidateCache(studioID)
	}
	return err
}

// Simulate traverses the tree against a test message and optional lead status.
func (s *Service) Simulate(ctx context.Context, studioID, treeID uuid.UUID, message, leadStatus string) (*SimulateResult, error) {
	tree, err := s.repo.GetTree(ctx, studioID, treeID)
	if err != nil {
		return nil, err
	}
	result := &SimulateResult{TraversalPath: []string{}}
	traverseNodes(tree.Nodes, strings.ToLower(message), leadStatus, result)
	return result, nil
}

// TraverseActiveTree finds the best active tree for the lead's pipeline status and
// traverses it against the message. Returns nil result if no matching tree is configured.
func (s *Service) TraverseActiveTree(ctx context.Context, studioID uuid.UUID, message, leadStatus string) (*SimulateResult, error) {
	tree, err := s.repo.GetActiveTreeForLead(ctx, studioID, leadStatus)
	if err == ErrTreeNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := &SimulateResult{TraversalPath: []string{}}
	traverseNodes(tree.Nodes, strings.ToLower(message), leadStatus, result)
	return result, nil
}

// traverseNodes walks the node tree depth-first matching all condition types.
func traverseNodes(nodes []Node, msgLower, leadStatus string, result *SimulateResult) bool {
	sentiment := detectSentiment(msgLower)
	intent := detectIntent(msgLower)

	for i := range nodes {
		n := &nodes[i]
		if n.ConditionType == ConditionDefault {
			continue
		}
		matched := false
		switch n.ConditionType {
		case ConditionKeyword:
			matched = matchKeyword(n, msgLower)
		case ConditionIntent:
			matched = matchIntent(n, intent)
		case ConditionSentiment:
			matched = matchSentiment(n, sentiment)
		case ConditionLeadStatus:
			matched = matchLeadStatus(n, leadStatus)
		}
		if matched {
			result.TraversalPath = append(result.TraversalPath, n.Label)
			if len(n.Children) > 0 {
				if traverseNodes(n.Children, msgLower, leadStatus, result) {
					return true
				}
			}
			applyNode(n, result)
			return true
		}
	}
	// Default catch-all.
	for i := range nodes {
		n := &nodes[i]
		if n.ConditionType == ConditionDefault {
			result.TraversalPath = append(result.TraversalPath, n.Label)
			applyNode(n, result)
			return true
		}
	}
	return false
}

// matchLeadStatus returns true if the lead's current status is in the node's allowed list.
// ConditionValue format: {"statuses": ["member","trial_booked"]}
func matchLeadStatus(n *Node, leadStatus string) bool {
	if leadStatus == "" {
		return false
	}
	v, ok := n.ConditionValue["statuses"]
	if !ok {
		return false
	}
	list, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		if strings.EqualFold(s, leadStatus) {
			return true
		}
	}
	return false
}

// detectSentiment returns "positive", "negative", or "neutral".
func detectSentiment(msgLower string) string {
	positiveWords := []string{"yes", "interested", "great", "love", "good", "perfect", "thanks", "thank you", "definitely", "sure", "count me in", "sign me up", "book it", "amazing", "excellent", "wonderful", "fantastic", "awesome"}
	negativeWords := []string{"no", "not interested", "bad", "hate", "no thanks", "never", "not now", "maybe later", "skip", "cancel", "terrible", "awful", "disappointed", "unhappy", "problem", "issue", "complaint", "refund", "angry"}
	pos, neg := 0, 0
	for _, w := range positiveWords {
		if strings.Contains(msgLower, w) {
			pos++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(msgLower, w) {
			neg++
		}
	}
	if pos > neg {
		return "positive"
	}
	if neg > pos {
		return "negative"
	}
	return "neutral"
}

// intentKeywords maps intent names to their trigger phrases.
var intentKeywords = map[string][]string{
	"pricing_question": {"price", "cost", "fee", "how much", "plans", "charges", "rate", "package", "membership cost", "subscription"},
	"booking_inquiry":  {"book", "trial", "visit", "schedule", "join", "sign up", "register", "enroll", "appointment", "session"},
	"complaint":        {"complaint", "unhappy", "disappointed", "terrible", "awful", "bad experience", "issue", "problem", "not working", "angry", "refund"},
	"greeting":         {"hello", "hi", "hey", "good morning", "good afternoon", "good evening", "howdy", "what's up"},
	"location":         {"where", "location", "address", "directions", "find you", "how to get", "branch", "gym location"},
	"hours":            {"hours", "open", "close", "timing", "when", "what time", "operating"},
	"off_topic":        {"weather", "news", "sports", "movie", "food", "restaurant"},
}

// detectIntent returns the best matching intent name for the message.
func detectIntent(msgLower string) string {
	best := ""
	bestScore := 0
	for intent, keywords := range intentKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(msgLower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = intent
		}
	}
	return best
}

func matchIntent(n *Node, detectedIntent string) bool {
	if detectedIntent == "" {
		return false
	}
	v, ok := n.ConditionValue["intent"]
	if !ok {
		return false
	}
	expected, ok := v.(string)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(expected), detectedIntent)
}

func matchSentiment(n *Node, detectedSentiment string) bool {
	v, ok := n.ConditionValue["sentiment"]
	if !ok {
		return false
	}
	expected, ok := v.(string)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(expected), detectedSentiment)
}

func matchKeyword(n *Node, msgLower string) bool {
	kws, ok := n.ConditionValue["keywords"]
	if !ok {
		return false
	}
	list, ok := kws.([]any)
	if !ok {
		return false
	}
	msgWords := strings.Fields(msgLower)
	for _, kw := range list {
		s, ok := kw.(string)
		if !ok {
			continue
		}
		keyword := strings.ToLower(strings.TrimSpace(s))
		if keyword == "" {
			continue
		}
		// Exact substring match first (handles multi-word keywords like "how much")
		if strings.Contains(msgLower, keyword) {
			return true
		}
		// Fuzzy match: check each word in the message against the keyword
		// allowing 1 typo for keywords ≥5 chars, 2 typos for keywords ≥8 chars
		maxDist := 0
		if len(keyword) >= 8 {
			maxDist = 2
		} else if len(keyword) >= 5 {
			maxDist = 1
		}
		if maxDist > 0 {
			for _, word := range msgWords {
				if levenshtein(word, keyword) <= maxDist {
					return true
				}
			}
		}
	}
	return false
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	// Skip if lengths differ too much — fast early exit
	diff := la - lb
	if diff < 0 {
		diff = -diff
	}
	if diff > 3 {
		return diff
	}
	row := make([]int, lb+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur := min3(row[j]+1, prev+1, row[j-1]+cost)
			row[j-1] = prev
			prev = cur
		}
		row[lb] = prev
	}
	return row[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func applyNode(n *Node, result *SimulateResult) {
	result.Matched = true
	result.NodeID = &n.ID
	result.NodeLabel = n.Label
	result.Reply = n.ReplyTemplate
	result.Action = n.Action
	if n.Action == ActionChangeStatus {
		if v, ok := n.ActionValue["target_status"]; ok {
			if s, ok := v.(string); ok {
				result.TargetStatus = s
			}
		}
	}
}

// ----- validation -----

func validateTree(input CreateTreeInput) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		errs["name"] = "required"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateNode(input CreateNodeInput) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(input.Label) == "" {
		errs["label"] = "required"
	}
	if !input.ConditionType.Valid() {
		errs["conditionType"] = "must be one of: keyword, intent, sentiment, default"
	}
	if !input.Action.Valid() {
		errs["action"] = "must be one of: reply, escalate_human, book_trial, send_link"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
