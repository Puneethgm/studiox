package crm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Executor calls any onboarded CRM's API for a given studio using only data
// (Provider, Operation, Connection) — no CRM-specific Go code. It's the
// generic replacement for internal/integrations/glofox.Client's one
// hand-written method per endpoint.
type Executor struct {
	repo   *Repo
	http   *http.Client
	tokens *tokenCache
}

func NewExecutor(repo *Repo) *Executor {
	return &Executor{repo: repo, http: &http.Client{Timeout: 15 * time.Second}, tokens: newTokenCache()}
}

// Execute calls the studio's active CRM connection's endpoint for
// operationKey, substituting params into the path/query/body per the
// operation's request_mapping, applying auth per the provider's auth_type,
// and extracting response_mapping fields from the reply.
func (e *Executor) Execute(ctx context.Context, studioID uuid.UUID, operationKey OperationKey, params map[string]any) (map[string]any, error) {
	conn, err := e.repo.GetActiveConnectionForStudio(ctx, studioID)
	if err != nil {
		return nil, fmt.Errorf("no active CRM connection for studio: %w", err)
	}
	provider, err := e.repo.GetProvider(ctx, conn.CRMProviderID)
	if err != nil {
		return nil, fmt.Errorf("load provider: %w", err)
	}
	op, err := e.repo.GetOperation(ctx, provider.ID, operationKey)
	if err != nil {
		return nil, fmt.Errorf("provider %q has no mapping for operation %q: %w", provider.Name, operationKey, err)
	}
	creds, err := e.repo.DecryptCredentials(conn)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}

	req, err := e.buildRequest(ctx, provider, op, params, creds)
	if err != nil {
		return nil, err
	}
	if err := e.applyAuth(ctx, req, provider, conn.ID, creds); err != nil {
		return nil, err
	}

	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", provider.Name, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s status %d: %s", provider.Name, resp.StatusCode, string(raw))
	}
	return mapResponse(raw, op.ResponseMapping)
}

// buildRequest applies request_mapping's "path"/"query"/"body" sections —
// each maps a CRM-side field name to the key in params we pull the value
// from — to build the outbound HTTP request. A "pathFromCreds" section
// substitutes from the connection's credentials instead of params, for CRMs
// (Glofox among them) that put a static per-account identifier — a branch
// or site ID — directly in the URL path rather than only in a header.
func (e *Executor) buildRequest(ctx context.Context, p *Provider, op *Operation, params map[string]any, creds map[string]string) (*http.Request, error) {
	path := op.PathTemplate
	if pathMap, ok := op.RequestMapping["path"].(map[string]any); ok {
		for crmField, paramKeyRaw := range pathMap {
			paramKey, _ := paramKeyRaw.(string)
			path = strings.ReplaceAll(path, "{"+crmField+"}", fmt.Sprint(params[paramKey]))
		}
	}
	if pathCredsMap, ok := op.RequestMapping["pathFromCreds"].(map[string]any); ok {
		for crmField, credKeyRaw := range pathCredsMap {
			credKey, _ := credKeyRaw.(string)
			path = strings.ReplaceAll(path, "{"+crmField+"}", creds[credKey])
		}
	}

	fullURL := strings.TrimRight(p.BaseURL, "/") + path
	if queryMap, ok := op.RequestMapping["query"].(map[string]any); ok && len(queryMap) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}
		q := u.Query()
		for crmField, paramKeyRaw := range queryMap {
			paramKey, _ := paramKeyRaw.(string)
			if v, ok := params[paramKey]; ok {
				q.Set(crmField, fmt.Sprint(v))
			}
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	var bodyReader io.Reader
	if bodyMap, ok := op.RequestMapping["body"].(map[string]any); ok && len(bodyMap) > 0 {
		body := map[string]any{}
		for crmField, paramKeyRaw := range bodyMap {
			paramKey, _ := paramKeyRaw.(string)
			if v, ok := params[paramKey]; ok {
				body[crmField] = v
			}
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(op.HTTPMethod), fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// applyAuth attaches credentials per the provider's auth_type. bearer /
// api_key / basic are static, one-shot header sets. token_exchange (e.g.
// Mindbody: static Api-Key/SiteId headers on every call, PLUS a short-lived
// bearer token obtained by logging in with those same credentials) sends
// the same static headers as api_key, then adds a cached-or-freshly-fetched
// Authorization: Bearer header on top.
func (e *Executor) applyAuth(ctx context.Context, req *http.Request, p *Provider, connectionID uuid.UUID, creds map[string]string) error {
	switch p.AuthType {
	case AuthBearer:
		token, ok := firstNonEmpty(creds, p.AuthFieldDefs)
		if !ok {
			return fmt.Errorf("%s: no bearer token configured", p.Name)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	case AuthAPIKey:
		setAPIKeyHeaders(req, p, creds)
	case AuthBasic:
		user := creds["username"]
		pass := creds["password"]
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	case AuthTokenExchange:
		setAPIKeyHeaders(req, p, creds)
		token, err := e.ensureToken(ctx, p, connectionID, creds)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	default:
		return fmt.Errorf("%s: unsupported auth_type %q", p.Name, p.AuthType)
	}
	return nil
}

// setAPIKeyHeaders sends every configured auth field as its own header,
// named by its key (e.g. Glofox's x-glofox-api-token / x-api-key /
// x-glofox-branch-id, or Mindbody's Api-Key / SiteId).
func setAPIKeyHeaders(req *http.Request, p *Provider, creds map[string]string) {
	for _, def := range p.AuthFieldDefs {
		if v, ok := creds[def.Key]; ok {
			req.Header.Set(def.Key, v)
		}
	}
}

// ensureToken returns a still-live cached bearer token for connectionID, or
// logs in fresh (per p's token_login_* config) and caches the result.
func (e *Executor) ensureToken(ctx context.Context, p *Provider, connectionID uuid.UUID, creds map[string]string) (string, error) {
	if token, ok := e.tokens.get(connectionID); ok {
		return token, nil
	}
	token, ttl, err := e.login(ctx, p, creds)
	if err != nil {
		return "", err
	}
	e.tokens.set(connectionID, token, ttl)
	return token, nil
}

// login performs a provider's token-exchange login step: POST (or whatever
// token_login_method says) to token_login_path with a body built from
// token_login_body_mapping, then pulls the token (and optionally its TTL)
// out of the JSON response per token_response_path / token_expiry_path.
func (e *Executor) login(ctx context.Context, p *Provider, creds map[string]string) (token string, ttl time.Duration, err error) {
	body := map[string]any{}
	for bodyField, mapping := range p.TokenLoginBodyMapping {
		if lit, ok := strings.CutPrefix(mapping, "const:"); ok {
			body[bodyField] = lit
			continue
		}
		body[bodyField] = creds[mapping]
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", 0, fmt.Errorf("marshal login body: %w", err)
	}

	method := p.TokenLoginMethod
	if method == "" {
		method = http.MethodPost
	}
	loginURL := strings.TrimRight(p.BaseURL, "/") + p.TokenLoginPath
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), loginURL, bytes.NewReader(b))
	if err != nil {
		return "", 0, fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKeyHeaders(req, p, creds) // e.g. Mindbody requires Api-Key/SiteId on the login call too

	resp, err := e.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("login status %d: %s", resp.StatusCode, string(raw))
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, fmt.Errorf("decode login response: %w", err)
	}
	token, _ = lookupPath(parsed, p.TokenResponsePath).(string)
	if token == "" {
		return "", 0, fmt.Errorf("login response had no token at %q", p.TokenResponsePath)
	}

	ttl = time.Duration(p.TokenExpirySeconds) * time.Second
	if p.TokenExpiryPath != "" {
		if secs, ok := toFloat(lookupPath(parsed, p.TokenExpiryPath)); ok {
			ttl = time.Duration(secs) * time.Second
		}
	}
	if ttl <= 0 {
		return "", 0, fmt.Errorf("provider has no usable token TTL (set token_expiry_seconds or token_expiry_path)")
	}
	return token, ttl, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

func firstNonEmpty(creds map[string]string, defs []AuthFieldDef) (string, bool) {
	for _, d := range defs {
		if v := creds[d.Key]; v != "" {
			return v, true
		}
	}
	return "", false
}

// mapResponse pulls response_mapping's {ourKey: crmField} pairs out of a
// JSON response. crmField supports dot-separated paths (e.g. "Client.Id")
// for CRMs that nest the created/fetched entity under a wrapper key —
// Mindbody's AddClient reply ({"Client":{"Id":...}, "Status":"Success"})
// being the case that first required it.
func mapResponse(raw []byte, mapping map[string]any) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(mapping) == 0 {
		return body, nil
	}
	out := map[string]any{}
	for ourKey, crmFieldRaw := range mapping {
		crmField, _ := crmFieldRaw.(string)
		out[ourKey] = lookupPath(body, crmField)
	}
	return out, nil
}

// lookupPath walks a dot-separated path ("Client.Id", or "Clients.0.Email"
// to index into an array — Mindbody's GetClients nests the match inside a
// list even for a single-ID lookup) through a decoded JSON value. Returns
// nil if any segment is missing or doesn't apply to the value at that point.
func lookupPath(body map[string]any, path string) any {
	var cur any = body
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			cur = v[idx]
		default:
			return nil
		}
	}
	return cur
}
