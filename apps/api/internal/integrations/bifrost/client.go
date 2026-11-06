// Package bifrost embeds Bifrost (https://docs.getbifrost.ai/) directly in
// this process via its Go core package (github.com/maximhq/bifrost/core) —
// no separate Bifrost service to run. This replaces the earlier HTTP-gateway
// incarnation of this package (a Docker container reached over
// 127.0.0.1:8090); see BIFROST_MIGRATION_PLAN.md for the call-site cutover
// plan, which is unaffected by this transport change.
//
// Bifrost has no provider keys of its own: every request carries the
// caller's own Groq/Gemini/Claude key (this app's existing per-studio BYOK
// model, unchanged) via a context value account.GetKeysForProvider reads —
// see directKeyContextKey below. One important consequence of that,
// confirmed against Bifrost's own docs and this package's earlier
// HTTP-gateway incarnation: a direct key only applies to the *first*
// provider in a request's fallback chain (including that provider's own
// retries) — a cross-provider fallback would have no key to use. So this
// client intentionally does NOT accept a cross-provider fallback list;
// Fallbacks is same-provider model fallback only (e.g. Groq's
// qwen3.8-27b -> gpt-oss-120b). Looping across providers (Groq -> Gemini ->
// Claude) stays the caller's responsibility, same as it is today.
package bifrost

import (
	"context"
	"fmt"
	"os"
	"strings"

	bifrostcore "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

// ProviderName mirrors internal/integrations/llm.ProviderName's three
// values, using Bifrost's own provider ids — Anthropic's id is "anthropic",
// not "claude" (this app's own package/provider-row name for it).
type ProviderName string

const (
	ProviderGroq      ProviderName = "groq"
	ProviderGemini    ProviderName = "gemini"
	ProviderAnthropic ProviderName = "anthropic"
	ProviderOpenAI    ProviderName = "openai"
)

var configuredProviders = []schemas.ModelProvider{
	schemas.Groq,
	schemas.Gemini,
	schemas.Anthropic,
	schemas.OpenAI,
}

// Reply bundles the text response with token usage, Bifrost's own computed
// cost for the call (CostUSD — Bifrost prices every request against its
// built-in provider rate catalog; 0 if a provider/model combination isn't
// in that catalog, not necessarily a free call), and (best-effort) which
// provider/model actually served the request — Bifrost reports the latter
// even when a model-level fallback fired, which is useful for LogLLMUsage to
// record what really happened rather than what was asked for.
type Reply struct {
	Text          string
	TokensIn      int
	TokensOut     int
	CostUSD       float64
	ServedByModel string // resolved_model_used, e.g. "qwen/qwen3.8-27b"
}

// Request is one chat-completion call. Provider+Model is the first model
// tried; Fallbacks are additional *same-provider* models tried in order if
// the first fails (see the package doc comment for why these can't cross
// providers while using a direct key). DirectKey is the caller's own raw
// provider key (a studio's groq_api_key/gemini_api_key/claude_api_key) — it
// is required: account.GetKeysForProvider errors on any request that
// doesn't carry one, since this account has no key pool of its own.
type Request struct {
	Provider     ProviderName
	Model        string
	Fallbacks    []string // bare model names, same provider as Model
	DirectKey    string
	SystemPrompt string // optional
	Prompt       string
	MaxTokens    int
	Temperature  *float64 // nil = let the provider default apply

	// ResponseFormat, when set, requests a JSON-schema-constrained reply
	// instead of free text — see ResponseFormatSchema. Used by
	// llmgateway.CompleteStructured (ai-reply-flow-llm-design task 3.1).
	ResponseFormat *ResponseFormatSchema
}

// ResponseFormatSchema is a provider-agnostic JSON-schema response-format
// request, translated into Bifrost's OpenAI-shaped
// response_format={"type":"json_schema","json_schema":{...}} wire format
// in Complete below. Schema should be a plain map/struct that marshals to
// a JSON Schema object (the usual "type":"object","properties":{...}
// shape) — Bifrost forwards it to whichever provider is called without
// reinterpreting it for providers that accept the OpenAI shape directly.
type ResponseFormatSchema struct {
	Name   string
	Schema any
	Strict bool
}

// Client wraps an in-process Bifrost instance — one per server process
// (New is called once at startup in main.go). Bifrost itself is safe for
// concurrent use across goroutines.
type Client struct {
	bf *bifrostcore.Bifrost
}

// directKeyContextKey is the context key account.GetKeysForProvider reads to
// build a single-key pool for the request currently in flight — see
// Complete below. Unexported so nothing outside this package can set it.
type directKeyContextKey struct{}

// account implements schemas.Account with no provider keys of its own:
// GetKeysForProvider always returns a single key built from whatever direct
// key the in-flight request carries (or errors if it has none). This is the
// Go-library equivalent of the file-config-only, empty-key-pool setup this
// replaces (deploy/bifrost/config.json), just expressed in code instead of
// JSON.
type account struct{}

func (account) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return configuredProviders, nil
}

func (account) GetKeysForProvider(ctx context.Context, providerKey schemas.ModelProvider) ([]schemas.Key, error) {
	directKey, _ := ctx.Value(directKeyContextKey{}).(string)
	if directKey == "" {
		return nil, fmt.Errorf("bifrost: no direct key on request context for provider %q", providerKey)
	}
	return []schemas.Key{{
		ID:     "direct",
		Name:   "direct",
		Value:  schemas.SecretVar{Val: directKey, SecretType: schemas.SecretTypePlainText},
		Models: schemas.WhiteList{"*"},
		Weight: 1,
	}}, nil
}

func (account) GetConfigForProvider(providerKey schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	return &schemas.ProviderConfig{}, nil
}

// New returns nil if BIFROST_ENABLED is explicitly set to "false" — same
// "nil client means not configured" convention internal/integrations/groq.New
// and internal/integrations/embeddings.New use, kept here as an escape hatch
// to disable Bifrost without a code change (resolve.go already branches on
// a nil client). Otherwise it always starts an embedded Bifrost instance —
// there's no separate service left that can be unreachable or misconfigured.
func New() *Client {
	if strings.EqualFold(os.Getenv("BIFROST_ENABLED"), "false") {
		return nil
	}
	bf, err := bifrostcore.Init(context.Background(), schemas.BifrostConfig{
		Account: account{},
	})
	if err != nil {
		// account{} above is never nil, so Init only fails here on a
		// Bifrost-internal bug — fail soft, same as an unreachable gateway
		// would have under the old HTTP client.
		return nil
	}
	return &Client{bf: bf}
}

// Complete sends one chat-completion request. It does not retry — Bifrost
// itself retries within a provider (see package doc comment), and a failure
// here is the caller's signal to try the next provider in its own loop,
// same shape the Groq/Gemini/Claude waterfall already has today.
func (c *Client) Complete(ctx context.Context, req Request) (Reply, error) {
	if c == nil {
		return Reply{}, fmt.Errorf("bifrost client not configured")
	}
	if req.Provider == "" || req.Model == "" {
		return Reply{}, fmt.Errorf("bifrost: provider and model are required")
	}
	if req.DirectKey == "" {
		return Reply{}, fmt.Errorf("bifrost: direct key is required")
	}

	messages := make([]schemas.ChatMessage, 0, 2)
	if req.SystemPrompt != "" {
		sysContent := req.SystemPrompt
		messages = append(messages, schemas.ChatMessage{
			Role:    schemas.ChatMessageRoleSystem,
			Content: &schemas.ChatMessageContent{ContentStr: &sysContent},
		})
	}
	prompt := req.Prompt
	messages = append(messages, schemas.ChatMessage{
		Role:    schemas.ChatMessageRoleUser,
		Content: &schemas.ChatMessageContent{ContentStr: &prompt},
	})

	provider := schemas.ModelProvider(req.Provider)

	fallbacks := make([]schemas.Fallback, 0, len(req.Fallbacks))
	for _, m := range req.Fallbacks {
		fallbacks = append(fallbacks, schemas.Fallback{Provider: provider, Model: m})
	}

	var params *schemas.ChatParameters
	if req.MaxTokens > 0 || req.Temperature != nil || req.ResponseFormat != nil {
		params = &schemas.ChatParameters{}
		if req.MaxTokens > 0 {
			params.MaxCompletionTokens = &req.MaxTokens
		}
		if req.Temperature != nil {
			params.Temperature = req.Temperature
		}
		if req.ResponseFormat != nil {
			// OpenAI-shaped response_format — the same plain
			// map[string]interface{} form schemas.ChatParameters'
			// doc comment says a Go caller builds (as opposed to the
			// raw-bytes form that comes off the wire from an actual
			// HTTP client). Bifrost forwards it to the provider as-is.
			format := map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   req.ResponseFormat.Name,
					"schema": req.ResponseFormat.Schema,
					"strict": req.ResponseFormat.Strict,
				},
			}
			var formatValue any = format
			params.ResponseFormat = &formatValue
		}
	}

	bfCtx := schemas.NewBifrostContextWithValue(ctx, schemas.NoDeadline, directKeyContextKey{}, req.DirectKey)

	resp, bfErr := c.bf.ChatCompletionRequest(bfCtx, &schemas.BifrostChatRequest{
		Provider:  provider,
		Model:     req.Model,
		Input:     messages,
		Params:    params,
		Fallbacks: fallbacks,
	})
	if bfErr != nil {
		msg := "unknown error"
		code := ""
		if bfErr.Error != nil {
			if bfErr.Error.Message != "" {
				msg = bfErr.Error.Message
			}
			if bfErr.Error.Code != nil {
				code = *bfErr.Error.Code
			}
		}
		status := 0
		if bfErr.StatusCode != nil {
			status = *bfErr.StatusCode
		}
		if code != "" {
			return Reply{}, fmt.Errorf("bifrost: %s/%s status %d: %s (%s)", req.Provider, req.Model, status, msg, code)
		}
		return Reply{}, fmt.Errorf("bifrost: %s/%s status %d: %s", req.Provider, req.Model, status, msg)
	}

	if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].ChatNonStreamResponseChoice == nil ||
		resp.Choices[0].ChatNonStreamResponseChoice.Message == nil ||
		resp.Choices[0].ChatNonStreamResponseChoice.Message.Content == nil ||
		resp.Choices[0].ChatNonStreamResponseChoice.Message.Content.ContentStr == nil ||
		strings.TrimSpace(*resp.Choices[0].ChatNonStreamResponseChoice.Message.Content.ContentStr) == "" {
		return Reply{}, fmt.Errorf("bifrost: %s/%s empty response", req.Provider, req.Model)
	}

	text := strings.TrimSpace(*resp.Choices[0].ChatNonStreamResponseChoice.Message.Content.ContentStr)

	reply := Reply{Text: text, ServedByModel: resp.ExtraFields.ResolvedModelUsed}
	if resp.Usage != nil {
		reply.TokensIn = resp.Usage.PromptTokens
		reply.TokensOut = resp.Usage.CompletionTokens
		if resp.Usage.Cost != nil {
			reply.CostUSD = resp.Usage.Cost.TotalCost
		}
	}
	return reply, nil
}
