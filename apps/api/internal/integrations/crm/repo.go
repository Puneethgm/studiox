package crm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/secrets"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

func NewRepo(pool *pgxpool.Pool, cipher *secrets.Cipher) *Repo {
	return &Repo{pool: pool, cipher: cipher}
}

// ============================================================
// crm_providers
// ============================================================

const providerColumns = `id, name, description, base_url, auth_type, auth_field_defs,
	token_login_path, token_login_method, token_login_body_mapping, token_response_path, token_expiry_path, token_expiry_seconds,
	spec_source, status, created_by, created_at, updated_at`

func (r *Repo) CreateProvider(ctx context.Context, p *Provider) error {
	fieldDefs, err := marshalJSONB(p.AuthFieldDefs)
	if err != nil {
		return fmt.Errorf("marshal auth field defs: %w", err)
	}
	loginBodyMapping, err := marshalJSONB(p.TokenLoginBodyMapping)
	if err != nil {
		return fmt.Errorf("marshal token login body mapping: %w", err)
	}
	loginMethod := p.TokenLoginMethod
	if loginMethod == "" {
		loginMethod = "POST"
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO crm_providers (
			name, description, base_url, auth_type, auth_field_defs,
			token_login_path, token_login_method, token_login_body_mapping, token_response_path, token_expiry_path, token_expiry_seconds,
			spec_source, status, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at
	`, p.Name, p.Description, p.BaseURL, p.AuthType, fieldDefs,
		p.TokenLoginPath, loginMethod, loginBodyMapping, p.TokenResponsePath, p.TokenExpiryPath, p.TokenExpirySeconds,
		p.SpecSource, p.Status, p.CreatedBy)
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *Repo) GetProvider(ctx context.Context, id uuid.UUID) (*Provider, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+providerColumns+` FROM crm_providers WHERE id = $1`, id)
	return scanProvider(row)
}

func (r *Repo) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+providerColumns+` FROM crm_providers ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var out []Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdateProviderInput carries every field a super-admin can correct before
// activating a draft provider — what the AI detected (base URL, auth type,
// auth field defs) plus, for auth_type = token_exchange, how to log in.
type UpdateProviderInput struct {
	Description           string
	BaseURL               string
	AuthType              AuthType
	AuthFieldDefs         []AuthFieldDef
	TokenLoginPath        string
	TokenLoginMethod      string
	TokenLoginBodyMapping map[string]string
	TokenResponsePath     string
	TokenExpiryPath       string
	TokenExpirySeconds    int
}

// UpdateProvider lets a super-admin correct what the AI detected before
// activating — the review step the plan requires between an AI-proposed
// mapping and it going live.
func (r *Repo) UpdateProvider(ctx context.Context, id uuid.UUID, in UpdateProviderInput) error {
	fieldDefs, err := marshalJSONB(in.AuthFieldDefs)
	if err != nil {
		return fmt.Errorf("marshal auth field defs: %w", err)
	}
	loginBodyMapping, err := marshalJSONB(in.TokenLoginBodyMapping)
	if err != nil {
		return fmt.Errorf("marshal token login body mapping: %w", err)
	}
	loginMethod := in.TokenLoginMethod
	if loginMethod == "" {
		loginMethod = "POST"
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE crm_providers
		SET description = $2, base_url = $3, auth_type = $4, auth_field_defs = $5,
		    token_login_path = $6, token_login_method = $7, token_login_body_mapping = $8,
		    token_response_path = $9, token_expiry_path = $10, token_expiry_seconds = $11,
		    updated_at = now()
		WHERE id = $1
	`, id, in.Description, in.BaseURL, in.AuthType, fieldDefs,
		in.TokenLoginPath, loginMethod, loginBodyMapping, in.TokenResponsePath, in.TokenExpiryPath, in.TokenExpirySeconds)
	if err != nil {
		return fmt.Errorf("update provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) ActivateProvider(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE crm_providers SET status = 'active', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("activate provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// marshalJSONB marshals v to a string, not []byte: the pool connects through
// PgBouncer in transaction-pool mode (see internal/platform/db.Connect),
// which forces pgx's simple_protocol query mode — under that mode a []byte
// parameter is sent as bytea, which fails against a jsonb column. A string
// is sent as text and casts to jsonb fine, matching the pattern already used
// in internal/messaging/repo.go and internal/studios/repo.go.
func marshalJSONB(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func scanProvider(row pgx.Row) (*Provider, error) {
	var p Provider
	var fieldDefs, loginBodyMapping []byte
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.BaseURL, &p.AuthType, &fieldDefs,
		&p.TokenLoginPath, &p.TokenLoginMethod, &loginBodyMapping, &p.TokenResponsePath, &p.TokenExpiryPath, &p.TokenExpirySeconds,
		&p.SpecSource, &p.Status, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan provider: %w", err)
	}
	if len(fieldDefs) > 0 {
		if err := json.Unmarshal(fieldDefs, &p.AuthFieldDefs); err != nil {
			return nil, fmt.Errorf("unmarshal auth field defs: %w", err)
		}
	}
	if len(loginBodyMapping) > 0 {
		if err := json.Unmarshal(loginBodyMapping, &p.TokenLoginBodyMapping); err != nil {
			return nil, fmt.Errorf("unmarshal token login body mapping: %w", err)
		}
	}
	return &p, nil
}

// ============================================================
// crm_operations
// ============================================================

func (r *Repo) CreateOperation(ctx context.Context, op *Operation) error {
	reqMapBytes, err := json.Marshal(op.RequestMapping)
	if err != nil {
		return fmt.Errorf("marshal request mapping: %w", err)
	}
	respMapBytes, err := json.Marshal(op.ResponseMapping)
	if err != nil {
		return fmt.Errorf("marshal response mapping: %w", err)
	}
	// See CreateProvider: string, not []byte, under simple_protocol.
	reqMap := string(reqMapBytes)
	respMap := string(respMapBytes)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO crm_operations (crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (crm_provider_id, operation_key) DO UPDATE
		SET http_method = EXCLUDED.http_method, path_template = EXCLUDED.path_template,
		    request_mapping = EXCLUDED.request_mapping, response_mapping = EXCLUDED.response_mapping,
		    reviewed = EXCLUDED.reviewed, updated_at = now()
		RETURNING id, created_at, updated_at
	`, op.CRMProviderID, op.OperationKey, op.HTTPMethod, op.PathTemplate, reqMap, respMap, op.Reviewed)
	return row.Scan(&op.ID, &op.CreatedAt, &op.UpdatedAt)
}

func (r *Repo) ListOperations(ctx context.Context, providerID uuid.UUID) ([]Operation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed, created_at, updated_at
		FROM crm_operations WHERE crm_provider_id = $1 ORDER BY operation_key
	`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	defer rows.Close()

	var out []Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *op)
	}
	return out, rows.Err()
}

// GetOperation looks up the single operation a studio's provider uses to
// fulfill key — the read the Executor does on every call.
func (r *Repo) GetOperation(ctx context.Context, providerID uuid.UUID, key OperationKey) (*Operation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, crm_provider_id, operation_key, http_method, path_template, request_mapping, response_mapping, reviewed, created_at, updated_at
		FROM crm_operations WHERE crm_provider_id = $1 AND operation_key = $2
	`, providerID, key)
	return scanOperation(row)
}

func scanOperation(row pgx.Row) (*Operation, error) {
	var op Operation
	var reqMap, respMap []byte
	if err := row.Scan(&op.ID, &op.CRMProviderID, &op.OperationKey, &op.HTTPMethod, &op.PathTemplate,
		&reqMap, &respMap, &op.Reviewed, &op.CreatedAt, &op.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan operation: %w", err)
	}
	if len(reqMap) > 0 {
		if err := json.Unmarshal(reqMap, &op.RequestMapping); err != nil {
			return nil, fmt.Errorf("unmarshal request mapping: %w", err)
		}
	}
	if len(respMap) > 0 {
		if err := json.Unmarshal(respMap, &op.ResponseMapping); err != nil {
			return nil, fmt.Errorf("unmarshal response mapping: %w", err)
		}
	}
	return &op, nil
}

// ============================================================
// crm_connections
// ============================================================

// CreateConnection encrypts credentials (a map of the provider's
// AuthFieldDefs keys to values) and stores the connection.
func (r *Repo) CreateConnection(ctx context.Context, studioID, providerID uuid.UUID, credentials map[string]string) (*Connection, error) {
	plain, err := json.Marshal(credentials)
	if err != nil {
		return nil, fmt.Errorf("marshal credentials: %w", err)
	}
	enc, err := r.cipher.Encrypt(string(plain))
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}
	c := &Connection{StudioID: studioID, CRMProviderID: providerID, CredentialsEnc: enc, Status: ConnectionActive}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO crm_connections (studio_id, crm_provider_id, credentials_enc, status)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (studio_id, crm_provider_id) DO UPDATE
		SET credentials_enc = EXCLUDED.credentials_enc, status = 'active', updated_at = now()
		RETURNING id, connected_at, updated_at
	`, studioID, providerID, enc, ConnectionActive)
	if err := row.Scan(&c.ID, &c.ConnectedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}
	return c, nil
}

// GetActiveConnection returns the studio's active connection for a provider,
// or ErrNotFound if none exists / it's been disconnected.
func (r *Repo) GetActiveConnection(ctx context.Context, studioID, providerID uuid.UUID) (*Connection, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, crm_provider_id, credentials_enc, status, connected_at, updated_at
		FROM crm_connections WHERE studio_id = $1 AND crm_provider_id = $2 AND status = 'active'
	`, studioID, providerID)
	var c Connection
	if err := row.Scan(&c.ID, &c.StudioID, &c.CRMProviderID, &c.CredentialsEnc, &c.Status, &c.ConnectedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active connection: %w", err)
	}
	return &c, nil
}

// GetActiveConnectionForStudio returns the studio's one active CRM
// connection. A studio connecting to more than one CRM at a time isn't a
// v1 use case, so callers (the Executor) only need a studio id, not a
// provider id, to know which CRM to call.
func (r *Repo) GetActiveConnectionForStudio(ctx context.Context, studioID uuid.UUID) (*Connection, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, studio_id, crm_provider_id, credentials_enc, status, connected_at, updated_at
		FROM crm_connections WHERE studio_id = $1 AND status = 'active'
		ORDER BY connected_at DESC LIMIT 1
	`, studioID)
	var c Connection
	if err := row.Scan(&c.ID, &c.StudioID, &c.CRMProviderID, &c.CredentialsEnc, &c.Status, &c.ConnectedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get active connection for studio: %w", err)
	}
	return &c, nil
}

// DecryptCredentials returns the {field_key: value} map stored on c.
func (r *Repo) DecryptCredentials(c *Connection) (map[string]string, error) {
	plain, err := r.cipher.Decrypt(c.CredentialsEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return out, nil
}

func (r *Repo) DisconnectConnection(ctx context.Context, studioID, providerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE crm_connections SET status = 'disconnected', updated_at = now()
		WHERE studio_id = $1 AND crm_provider_id = $2
	`, studioID, providerID)
	if err != nil {
		return fmt.Errorf("disconnect connection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
