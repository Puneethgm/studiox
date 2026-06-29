package decisiontree

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTreeNotFound = errors.New("decision tree not found")
	ErrNodeNotFound = errors.New("tree node not found")
)

type ConditionType string

const (
	ConditionKeyword    ConditionType = "keyword"
	ConditionIntent     ConditionType = "intent"
	ConditionSentiment  ConditionType = "sentiment"
	ConditionDefault    ConditionType = "default"
	ConditionLeadStatus ConditionType = "lead_status"
)

func (c ConditionType) Valid() bool {
	switch c {
	case ConditionKeyword, ConditionIntent, ConditionSentiment, ConditionDefault, ConditionLeadStatus:
		return true
	}
	return false
}

type Action string

const (
	ActionReply         Action = "reply"
	ActionEscalate      Action = "escalate_human"
	ActionBookTrial     Action = "book_trial"
	ActionSendLink      Action = "send_link"
	ActionChangeStatus  Action = "change_status"
)

func (a Action) Valid() bool {
	switch a {
	case ActionReply, ActionEscalate, ActionBookTrial, ActionSendLink, ActionChangeStatus:
		return true
	}
	return false
}

type Tree struct {
	ID             uuid.UUID `json:"id"`
	StudioID       uuid.UUID `json:"studioId"`
	Name           string    `json:"name"`
	IsActive       bool      `json:"isActive"`
	// TargetStatuses limits which lead pipeline stages this tree responds to.
	// Empty = responds to all leads. Non-empty = only those statuses.
	TargetStatuses []string  `json:"targetStatuses"`
	Nodes          []Node    `json:"nodes,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ConditionValue holds the configuration for a node's condition.
// For keyword: {"keywords": ["price","cost"]}
// For intent:  {"intent": "pricing"}
// For sentiment: {"sentiment": "negative"}
// For default: {}
type ConditionValue map[string]any

type Node struct {
	ID             uuid.UUID      `json:"id"`
	TreeID         uuid.UUID      `json:"treeId"`
	ParentID       *uuid.UUID     `json:"parentId"`
	Label          string         `json:"label"`
	ConditionType  ConditionType  `json:"conditionType"`
	ConditionValue ConditionValue `json:"conditionValue"`
	ReplyTemplate  string         `json:"replyTemplate"`
	Action         Action         `json:"action"`
	// ActionValue stores action-specific config, e.g. {"target_status":"member"} for change_status.
	ActionValue    ConditionValue `json:"actionValue"`
	SortOrder      int            `json:"sortOrder"`
	Children       []Node         `json:"children,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type CreateTreeInput struct {
	Name           string
	TargetStatuses []string
}

type UpdateTreeInput struct {
	Name           *string
	IsActive       *bool
	TargetStatuses []string // nil = don't change; empty slice = clear to all leads
	UpdateStatuses bool     // must be true to update TargetStatuses (distinguishes nil vs empty)
}

type CreateNodeInput struct {
	TreeID         uuid.UUID
	ParentID       *uuid.UUID
	Label          string
	ConditionType  ConditionType
	ConditionValue ConditionValue
	ReplyTemplate  string
	Action         Action
	ActionValue    ConditionValue
	SortOrder      int
}

type UpdateNodeInput struct {
	Label          *string
	ConditionType  *ConditionType
	ConditionValue ConditionValue
	ReplyTemplate  *string
	Action         *Action
	ActionValue    ConditionValue
	SortOrder      *int
}

// SimulateResult holds which node fired for a test message.
type SimulateResult struct {
	Matched       bool       `json:"matched"`
	NodeID        *uuid.UUID `json:"nodeId,omitempty"`
	NodeLabel     string     `json:"nodeLabel,omitempty"`
	Reply         string     `json:"reply,omitempty"`
	Action        Action     `json:"action,omitempty"`
	// TargetStatus is set when Action == ActionChangeStatus.
	TargetStatus  string     `json:"targetStatus,omitempty"`
	TraversalPath []string   `json:"traversalPath"`
}
