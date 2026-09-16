package crm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/projectx/api/internal/integrations/llm"
)

// ParsePurpose is the ai_task_configs.purpose key super-admin configures a
// provider/model for, to drive ParseCRMDoc.
const ParsePurpose = "crm_doc_parsing"

// maxDocSize caps how much of an uploaded spec gets sent to the LLM. Large
// enough for a real single-service OpenAPI doc; chunking a doc that
// genuinely exceeds this is a follow-up, not a v1 requirement.
const maxDocSize = 300_000 // ~300KB of text

// parsedProvider is the JSON shape ParseCRMDoc asks the LLM to return.
type parsedProvider struct {
	BaseURL       string                      `json:"baseUrl"`
	AuthType      string                      `json:"authType"`
	AuthFieldDefs []AuthFieldDef              `json:"authFieldDefs"`
	Operations    map[string]*parsedOperation `json:"operations"`

	// Only present when AuthType == "token_exchange" — see Provider's Token*
	// fields for exactly what each means.
	TokenLoginPath        string            `json:"tokenLoginPath"`
	TokenLoginMethod      string            `json:"tokenLoginMethod"`
	TokenLoginBodyMapping map[string]string `json:"tokenLoginBodyMapping"`
	TokenResponsePath     string            `json:"tokenResponsePath"`
	TokenExpiryPath       string            `json:"tokenExpiryPath"`
	TokenExpirySeconds    int               `json:"tokenExpirySeconds"`
}

type parsedOperation struct {
	HTTPMethod      string         `json:"httpMethod"`
	PathTemplate    string         `json:"pathTemplate"`
	RequestMapping  map[string]any `json:"requestMapping"`
	ResponseMapping map[string]any `json:"responseMapping"`
}

// ParseCRMDoc sends docText to provider (the caller resolves which LLM that
// is via llm.Resolver.ResolveProvider(ctx, ParsePurpose) — kept out of this
// function so it stays testable with a fake llm.Provider, no DB or real API
// call needed), asking it to map the CRM's endpoints onto the fixed
// operations Catalog, then persists the result as a draft provider (plus
// one crm_operations row per operation the LLM found — never all six,
// since most CRMs won't expose every one of them). The provider stays
// status=draft, every operation reviewed=false, until a super-admin
// reviews and activates it via the admin UI.
func ParseCRMDoc(ctx context.Context, provider llm.Provider, repo *Repo, name, docText string) (*Provider, []Operation, error) {
	if len(docText) > maxDocSize {
		docText = docText[:maxDocSize]
	}

	reply, err := provider.Analyze(ctx, buildSystemPrompt(), docText)
	if err != nil {
		return nil, nil, fmt.Errorf("analyze document: %w", err)
	}

	parsed, err := parseReplyJSON(reply.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("parse LLM response: %w", err)
	}

	p := &Provider{
		Name:                  name,
		BaseURL:               parsed.BaseURL,
		AuthType:              AuthType(parsed.AuthType),
		AuthFieldDefs:         parsed.AuthFieldDefs,
		TokenLoginPath:        parsed.TokenLoginPath,
		TokenLoginMethod:      parsed.TokenLoginMethod,
		TokenLoginBodyMapping: parsed.TokenLoginBodyMapping,
		TokenResponsePath:     parsed.TokenResponsePath,
		TokenExpiryPath:       parsed.TokenExpiryPath,
		TokenExpirySeconds:    parsed.TokenExpirySeconds,
		SpecSource:            docText,
		Status:                ProviderDraft,
	}
	if err := repo.CreateProvider(ctx, p); err != nil {
		return nil, nil, fmt.Errorf("save draft provider: %w", err)
	}

	var ops []Operation
	for _, spec := range Catalog {
		po, ok := parsed.Operations[string(spec.Key)]
		if !ok || po == nil || po.PathTemplate == "" {
			continue // LLM didn't find this operation in the doc — fine, not every CRM has all six
		}
		op := Operation{
			CRMProviderID:   p.ID,
			OperationKey:    spec.Key,
			HTTPMethod:      po.HTTPMethod,
			PathTemplate:    po.PathTemplate,
			RequestMapping:  po.RequestMapping,
			ResponseMapping: po.ResponseMapping,
			Reviewed:        false,
		}
		if err := repo.CreateOperation(ctx, &op); err != nil {
			return nil, nil, fmt.Errorf("save operation %q: %w", spec.Key, err)
		}
		ops = append(ops, op)
	}

	return p, ops, nil
}

func buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are mapping a CRM's API documentation onto a fixed set of operations for a fitness-studio SaaS platform. ")
	b.WriteString("Read the provided document (OpenAPI/Swagger, or free-form API docs) and, for each operation below, find the ")
	b.WriteString("single best-matching endpoint if one exists in the document — do not invent an endpoint that isn't described.\n\n")
	b.WriteString("Operations:\n")
	for _, spec := range Catalog {
		fmt.Fprintf(&b, "- %s: %s (params: %s)\n", spec.Key, spec.Description, strings.Join(spec.Params, ", "))
	}
	b.WriteString("\nAlso detect the CRM's base URL and authentication scheme — one of \"bearer\", \"api_key\", \"basic\", or \"token_exchange\". ")
	b.WriteString("For api_key, list every required header as an auth field (key = header name). ")
	b.WriteString("Use \"token_exchange\" only when the document describes a login step that returns a short-lived bearer token in exchange for ")
	b.WriteString("credentials (e.g. a POST to a token/login endpoint) — separate from any static headers still required on every other call. ")
	b.WriteString("For token_exchange, also fill in tokenLoginPath (relative to baseUrl), tokenLoginMethod, tokenLoginBodyMapping (the login request ")
	b.WriteString("body: each value is either the key of one of authFieldDefs, or a literal constant prefixed \"const:\", e.g. {\"Username\":\"const:Siteowner\",\"Password\":\"api_key\"}), ")
	b.WriteString("tokenResponsePath (dot path to the token in the login response, e.g. \"AccessToken\"), and either tokenExpiryPath (dot path to the token's ")
	b.WriteString("lifetime in seconds in that same response) or tokenExpirySeconds (a fixed fallback) if the document states one.\n\n")
	b.WriteString("Respond with ONLY a JSON object, no prose, no markdown fences, matching exactly this shape:\n")
	b.WriteString(`{
  "baseUrl": "https://api.example.com",
  "authType": "bearer" | "api_key" | "basic" | "token_exchange",
  "authFieldDefs": [{"key": "...", "label": "...", "secret": true}],
  "tokenLoginPath": "", "tokenLoginMethod": "", "tokenLoginBodyMapping": {}, "tokenResponsePath": "", "tokenExpiryPath": "", "tokenExpirySeconds": 0,
  "operations": {
    "create_lead": {"httpMethod": "POST", "pathTemplate": "/leads", "requestMapping": {"body": {"crmFieldName": "ourParamName"}}, "responseMapping": {"ourKey": "crmResponseField"}},
    "...": null
  }
}`)
	b.WriteString("\n\nOmit or set to null any operation the document doesn't describe — do not guess. Leave the token* fields at their zero values unless authType is token_exchange.")
	return b.String()
}

// parseReplyJSON strips a markdown code fence if the LLM wrapped its JSON
// in one despite being asked not to (common enough across providers to
// handle defensively rather than fail the whole parse over it).
func parseReplyJSON(text string) (*parsedProvider, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var out parsedProvider
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON in LLM reply: %w", err)
	}
	return &out, nil
}
