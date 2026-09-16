// Package crm is a generic, database-driven CRM integration framework.
// It replaces the pattern used by internal/integrations/glofox (one
// hardcoded Go client, one global platform-wide credential set) with: a
// super-admin-defined catalog of CRM "providers" and the operations each
// one supports (Provider/Operation, this file), a per-studio encrypted
// credential vault (Connection), and a generic Executor that calls any
// provider's API using only that data — no CRM-specific Go code required
// to onboard a new CRM or connect a new studio to one.
package crm

import (
	"time"

	"github.com/google/uuid"
)

type AuthType string

const (
	AuthBearer AuthType = "bearer"
	AuthAPIKey AuthType = "api_key"
	AuthBasic  AuthType = "basic"
	// AuthTokenExchange is for CRMs (e.g. Mindbody) whose calls need the same
	// static headers as AuthAPIKey PLUS a short-lived bearer token obtained
	// by logging in with those credentials — see Provider's Token* fields.
	AuthTokenExchange AuthType = "token_exchange"
)

type ProviderStatus string

const (
	ProviderDraft  ProviderStatus = "draft"
	ProviderActive ProviderStatus = "active"
)

// AuthFieldDef describes one credential field a provider's auth needs (e.g.
// Glofox needs three: api_key, api_token, branch_id). Drives both the
// dynamic "connect a studio" form on the frontend and what the Executor
// expects to find in a Connection's decrypted credentials.
type AuthFieldDef struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Secret bool   `json:"secret"`
}

// Provider is one onboarded CRM (Glofox, Mindbody, ...). Platform-wide —
// the shape of a CRM's API isn't studio-scoped, only its credentials are.
type Provider struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	BaseURL       string         `json:"baseUrl"`
	AuthType      AuthType       `json:"authType"`
	AuthFieldDefs []AuthFieldDef `json:"authFieldDefs"`

	// Token-exchange fields — only meaningful when AuthType == AuthTokenExchange.
	// See migrations/20260912000003_crm_token_exchange_auth.sql for the
	// exact semantics of each.
	TokenLoginPath        string            `json:"tokenLoginPath,omitempty"`
	TokenLoginMethod      string            `json:"tokenLoginMethod,omitempty"`
	TokenLoginBodyMapping map[string]string `json:"tokenLoginBodyMapping,omitempty"`
	TokenResponsePath     string            `json:"tokenResponsePath,omitempty"`
	TokenExpiryPath       string            `json:"tokenExpiryPath,omitempty"`
	TokenExpirySeconds    int               `json:"tokenExpirySeconds,omitempty"`

	SpecSource string         `json:"specSource,omitempty"`
	Status     ProviderStatus `json:"status"`
	CreatedBy  *uuid.UUID     `json:"createdBy,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// Operation is one (provider, operation_key) mapping — which endpoint on
// that CRM fulfills one of the fixed OperationKey values in operations.go.
type Operation struct {
	ID              uuid.UUID      `json:"id"`
	CRMProviderID   uuid.UUID      `json:"crmProviderId"`
	OperationKey    OperationKey   `json:"operationKey"`
	HTTPMethod      string         `json:"httpMethod"`
	PathTemplate    string         `json:"pathTemplate"`
	RequestMapping  map[string]any `json:"requestMapping"`
	ResponseMapping map[string]any `json:"responseMapping"`
	Reviewed        bool           `json:"reviewed"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type ConnectionStatus string

const (
	ConnectionActive       ConnectionStatus = "active"
	ConnectionDisconnected ConnectionStatus = "disconnected"
)

// Connection is one studio's credentials for one provider. Credentials are
// stored encrypted (Cipher.Encrypt) as a JSON blob of {field_key: value}
// matching the provider's AuthFieldDefs — decrypted only inside the
// Executor, never returned from a list/get endpoint.
type Connection struct {
	ID             uuid.UUID        `json:"id"`
	StudioID       uuid.UUID        `json:"studioId"`
	CRMProviderID  uuid.UUID        `json:"crmProviderId"`
	CredentialsEnc string           `json:"-"`
	Status         ConnectionStatus `json:"status"`
	ConnectedAt    time.Time        `json:"connectedAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}
