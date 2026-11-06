// Package llmgateway is the single entry point every AI feature calls for
// an LLM completion — "I need a reply/summary/classification for this
// prompt," never "call Groq with this key." It owns per-feature routing
// (ai_feature_routes, cached in memory), the provider fallback loop,
// output cleanup (reasoning-block stripping, empty-after-cleanup
// detection), the per-feature time budget, usage/cost attribution, and
// failing-key detection. Bifrost itself (internal/integrations/bifrost)
// stays a thin wrapper around the embedded Bifrost library; this package
// is the policy layer above it. See
// openspec/changes/llm-gateway-and-key-management/design.md.
package llmgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/bifrost"
	"github.com/projectx/api/internal/integrations/llm"
	"github.com/projectx/api/internal/studios"
)

// Feature identifies which AI capability is asking for a completion. A
// closed set of compiled-in constants, not a free-form string — see
// design.md Decision 2: adding a feature always requires a new call site
// in Go anyway, so there's no value in letting the DB invent new ones.
type Feature string

const (
	FeatureCustomerReply    Feature = "customer_reply"
	FeatureSummary          Feature = "summary"
	FeatureStyleProfile     Feature = "style_profile"
	FeatureTestChat         Feature = "test_chat"
	FeatureTemplateGen      Feature = "template_gen"
	FeatureEnableAIFollowup Feature = "enable_ai_followup"
)

// bifrostProviderNames maps this package's plain provider identifiers
// (also used by ai_feature_routes.route.providers and
// studios.GetPlatformAIProviderKey) to bifrost's and llm's own provider
// enums — the same three-way mapping ai_models_http.go's
// chatReplyProviders and ai_worker.go's per-call-site constants already
// duplicate; kept local here rather than importing internal/studios'
// unexported map.
var bifrostProviderNames = map[string]bifrost.ProviderName{
	"groq":   bifrost.ProviderGroq,
	"gemini": bifrost.ProviderGemini,
	"claude": bifrost.ProviderAnthropic,
	"openai": bifrost.ProviderOpenAI,
}

var llmProviderNames = map[string]llm.ProviderName{
	"groq":   llm.ProviderGroq,
	"gemini": llm.ProviderGemini,
	"claude": llm.ProviderClaude,
	"openai": llm.ProviderOpenAI,
}

// routeCacheTTL bounds how stale an in-memory route can be after a
// superadmin edits it — see design.md Decision 3 (no push-invalidation
// infrastructure exists elsewhere in this codebase; a short poll-on-read
// TTL matches the existing platform_ai_models pattern).
const routeCacheTTL = 30 * time.Second

// authFailurePattern and failingKeyConsecutiveThreshold mirror
// internal/messaging's identically-named definitions (see
// ai_worker.go) — duplicated here rather than exported cross-package
// because messaging's copies belong to the old waterfall code being
// migrated away from in this same change (deleted in Phase 3 cleanup);
// this is the new, permanent home for the logic.
var authFailurePattern = regexp.MustCompile(`(?i)\b(401|unauthorized|invalid[_ -]?api[_ -]?key|incorrect api key|authentication[_ ]?(failed|error)|invalid x-api-key|invalid_request_error.*api[_ -]?key)\b`)

const failingKeyConsecutiveThreshold = 3

// reasoningBlock mirrors internal/messaging's identically-named pattern —
// see that package's doc comment; same duplication rationale as
// authFailurePattern above.
var reasoningBlock = regexp.MustCompile(`(?is)<(?:think|thinking|reasoning)>.*?</\s*(?:think|thinking|reasoning)\s*>`)

func stripReasoningText(text string) string {
	return strings.TrimSpace(reasoningBlock.ReplaceAllString(text, ""))
}

type cachedRoute struct {
	route     Route
	expiresAt time.Time
}

// bifrostCompleter is the one method of *bifrost.Client this package
// calls — narrowed to an interface so tests can inject a fake and
// deterministically exercise the fallback loop, time budget, and output
// cleanup without a live provider call. *bifrost.Client satisfies this
// automatically; production code always passes the real thing via New.
type bifrostCompleter interface {
	Complete(ctx context.Context, req bifrost.Request) (bifrost.Reply, error)
}

// Client is the gateway itself — construct one per process (same lifetime
// as the bifrost.Client/studios.Repo it wraps) and share it across every
// AI feature call site.
type Client struct {
	bifrostClient bifrostCompleter
	llmRepo       *llm.Repo
	studiosRepo   *studios.Repo
	repo          *Repo
	log           *slog.Logger

	mu         sync.RWMutex
	routeCache map[Feature]cachedRoute
}

func New(bifrostClient *bifrost.Client, llmRepo *llm.Repo, studiosRepo *studios.Repo, repo *Repo, log *slog.Logger) *Client {
	return &Client{
		bifrostClient: bifrostClient,
		llmRepo:       llmRepo,
		studiosRepo:   studiosRepo,
		repo:          repo,
		log:           log,
		routeCache:    make(map[Feature]cachedRoute),
	}
}

// NewForTest is New with the completer narrowed to bifrostCompleter
// instead of the concrete *bifrost.Client, so tests in other packages
// (e.g. internal/messaging's real handleMessage-path tests) can inject a
// deterministic, call-counting fake in place of a live provider call —
// the same technique this package's own gateway_test.go fakeCompleter
// uses internally, exposed for cross-package test use rather than
// duplicating Client's unexported fields.
func NewForTest(bifrostClient bifrostCompleter, llmRepo *llm.Repo, studiosRepo *studios.Repo, repo *Repo, log *slog.Logger) *Client {
	return &Client{
		bifrostClient: bifrostClient,
		llmRepo:       llmRepo,
		studiosRepo:   studiosRepo,
		repo:          repo,
		log:           log,
		routeCache:    make(map[Feature]cachedRoute),
	}
}

func (c *Client) getRoute(ctx context.Context, feature Feature) (Route, error) {
	c.mu.RLock()
	cached, ok := c.routeCache[feature]
	c.mu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.route, nil
	}

	route, err := c.repo.GetRoute(ctx, feature)
	if err != nil {
		return Route{}, err
	}

	c.mu.Lock()
	c.routeCache[feature] = cachedRoute{route: route, expiresAt: time.Now().Add(routeCacheTTL)}
	c.mu.Unlock()
	return route, nil
}

// isUsable is the default "did this provider actually answer" check: a
// non-empty reply after reasoning-block cleanup. Complete applies route's
// MinUsableLength on top when set (the customer-reply path's "a short
// Groq reply is a soft failure" rule, now config instead of a hardcoded
// >=15 in Go).
func isUsable(text string, minLength int) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return len(text) >= minLength
}

// Complete is the one door every AI feature calls through. It resolves
// feature's route, tries each configured provider in order (each
// provider's enabled models tried via llm.Repo.EnabledModels, same as
// before this gateway existed), stops at the first usable reply, and logs
// usage for every attempt. Returns ErrNoRoute if the feature has no
// routing config, or the zero text/sourceRef (not an error) if every
// provider failed or is unconfigured — matching the old waterfall
// functions' contract, so callers can tell "no route" (a config bug) from
// "every provider failed" (a runtime condition fail-loud handling reacts
// to) without a type assertion.
func (c *Client) Complete(ctx context.Context, feature Feature, studioID uuid.UUID, prompt string) (text, sourceRef string, err error) {
	if c == nil {
		return "", "", nil
	}
	route, err := c.getRoute(ctx, feature)
	if err != nil {
		return "", "", err
	}

	budget := route.TimeBudgetMS
	if budget <= 0 {
		budget = 60000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()

	for _, provider := range route.Providers {
		key, keyErr := c.studiosRepo.GetPlatformAIProviderKey(ctx, provider)
		if keyErr != nil || key == "" {
			continue
		}
		bfProvider, ok := bifrostProviderNames[provider]
		if !ok {
			continue
		}
		llmProvider, ok := llmProviderNames[provider]
		if !ok {
			continue
		}
		maxTokens := route.MaxTokens[provider]
		var temperature *float64
		if t, ok := route.Temperature[provider]; ok {
			temperature = &t
		}

		if text, ref := c.attemptProvider(ctx, studioID, feature, provider, llmProvider, bfProvider, key, prompt, maxTokens, temperature, route.MinUsableLength, nil); ref != "" {
			return text, ref, nil
		}
	}
	return "", "", nil
}

// CompleteStructured is Complete's structured-output counterpart
// (ai-reply-flow-llm-design task 3.1/design.md "Structured output"
// decision): it asks the provider for a JSON-schema-constrained reply
// (via Bifrost's native response-format support, not a second prompt
// asking the model to "reply in JSON") and unmarshals the result into
// T. Same route/fallback/usage-logging machinery as Complete — this is
// an additive entry point, not a parallel gateway.
//
// Returns ErrNoRoute if the feature has no routing config. If every
// provider fails/is unconfigured, or a provider's response doesn't parse
// as valid T, returns the zero T and a non-nil error — unlike Complete,
// a structured caller cannot treat "no text back" as silently
// equivalent to "parsed into nothing," so this does not follow
// Complete's "empty return, no error" contract.
func CompleteStructured[T any](ctx context.Context, c *Client, feature Feature, studioID uuid.UUID, prompt string, schemaName string, schema any) (T, string, error) {
	var zero T
	if c == nil {
		return zero, "", fmt.Errorf("llmgateway: client not configured")
	}
	route, err := c.getRoute(ctx, feature)
	if err != nil {
		return zero, "", err
	}

	budget := route.TimeBudgetMS
	if budget <= 0 {
		budget = 60000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()

	format := &bifrost.ResponseFormatSchema{Name: schemaName, Schema: schema, Strict: true}

	var lastParseErr error
	for _, provider := range route.Providers {
		key, keyErr := c.studiosRepo.GetPlatformAIProviderKey(ctx, provider)
		if keyErr != nil || key == "" {
			continue
		}
		bfProvider, ok := bifrostProviderNames[provider]
		if !ok {
			continue
		}
		llmProvider, ok := llmProviderNames[provider]
		if !ok {
			continue
		}
		maxTokens := route.MaxTokens[provider]
		var temperature *float64
		if t, ok := route.Temperature[provider]; ok {
			temperature = &t
		}

		text, ref := c.attemptProvider(ctx, studioID, feature, provider, llmProvider, bfProvider, key, prompt, maxTokens, temperature, route.MinUsableLength, format)
		if ref == "" {
			continue
		}
		var parsed T
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			// A provider claimed success (isUsable passed — some non-empty
			// text came back) but it wasn't valid JSON for T. Try the next
			// provider in the route rather than failing the whole call on
			// one provider's malformed structured output.
			lastParseErr = fmt.Errorf("provider %s returned text that doesn't parse as %T: %w", provider, parsed, err)
			if c.log != nil {
				c.log.Warn("llm structured call: response failed schema parse, trying next provider",
					"feature", feature, "studio_id", studioID, "provider", provider, "err", err)
			}
			continue
		}
		return parsed, ref, nil
	}
	if lastParseErr != nil {
		return zero, "", lastParseErr
	}
	return zero, "", fmt.Errorf("llmgateway: all providers failed or unconfigured for feature %q", feature)
}

func (c *Client) attemptProvider(ctx context.Context, studioID uuid.UUID, feature Feature, providerName string, llmProvider llm.ProviderName, bfProvider bifrost.ProviderName, directKey, prompt string, maxTokens int, temperature *float64, minUsableLength int, responseFormat *bifrost.ResponseFormatSchema) (text, sourceRef string) {
	models, _ := c.llmRepo.EnabledModels(ctx, llmProvider)
	for _, model := range models {
		t0 := time.Now()
		r, err := c.bifrostClient.Complete(ctx, bifrost.Request{
			Provider:       bfProvider,
			Model:          model,
			DirectKey:      directKey,
			ResponseFormat: responseFormat,
			Prompt:         prompt,
			MaxTokens:      maxTokens,
			Temperature:    temperature,
		})
		latMs := int(time.Since(t0).Milliseconds())
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		r.Text = stripReasoningText(r.Text)
		ok := err == nil && isUsable(r.Text, minUsableLength)
		c.repo.LogUsage(ctx, studioID, feature, providerName, model, latMs, ok, errMsg, r.TokensIn, r.TokensOut, r.CostUSD, false)
		if c.log != nil {
			c.log.Info("llm call",
				"component", "llm_usage",
				"feature", feature,
				"studio_id", studioID,
				"provider", providerName,
				"model", model,
				"served_by_model", r.ServedByModel,
				"latency_ms", latMs,
				"success", ok,
				"tokens_in", r.TokensIn,
				"tokens_out", r.TokensOut,
				"cost_usd", r.CostUSD,
				"err", errMsg,
			)
		}
		if err != nil && authFailurePattern.MatchString(errMsg) {
			c.checkAndFlagFailingKey(ctx, providerName)
		}
		if ok {
			return r.Text, providerName + ":" + model
		}
	}
	return "", ""
}

// LogCacheHit records that feature's call for studioID was served from a
// cache (e.g. internal/messaging.AnswerCache) instead of a live provider
// call — the gateway doesn't own caching itself (that stays the caller's
// concern, same as before this package existed), but usage attribution
// needs to know the difference per the usage-tracking spec's "Cache hits
// are distinguished from live calls" requirement.
func (c *Client) LogCacheHit(ctx context.Context, studioID uuid.UUID, feature Feature) {
	if c == nil {
		return
	}
	c.repo.LogUsage(ctx, studioID, feature, "", "", 0, true, "", 0, 0, 0, true)
}

func (c *Client) checkAndFlagFailingKey(ctx context.Context, provider string) {
	allFailed, err := c.repo.RecentCallsAllFailed(ctx, provider, failingKeyConsecutiveThreshold)
	if err != nil {
		if c.log != nil {
			c.log.Warn("check recent llm calls for failing key", "provider", provider, "err", err)
		}
		return
	}
	if !allFailed {
		return
	}
	if err := c.studiosRepo.MarkPlatformAIProviderKeyFailing(ctx, provider); err != nil {
		if c.log != nil {
			c.log.Warn("mark platform ai key failing", "provider", provider, "err", err)
		}
		return
	}
	c.repo.RecordAlert(ctx, "key_failing", map[string]any{"provider": provider}, uuid.Nil)
}
