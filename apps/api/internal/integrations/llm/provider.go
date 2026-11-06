package llm

import (
	"context"

	"github.com/projectx/api/internal/integrations/bifrost"
)

// Reply is the common shape every adapter normalizes its underlying
// client's response into.
type Reply struct {
	Text      string
	TokensIn  int
	TokensOut int
}

// Provider is the one thing internal/integrations/crm's doc-parsing step
// depends on — never a specific client package directly, so swapping which
// LLM handles a task is a settings change (ai_task_configs), not a code
// change.
type Provider interface {
	Analyze(ctx context.Context, systemPrompt, document string) (Reply, error)
}

// bifrostAdapter routes a task through the Bifrost gateway
// (internal/integrations/bifrost) instead of a provider-specific client —
// one adapter for all three providers, since Bifrost's OpenAI-compatible
// endpoint fronts all of them. See BIFROST_MIGRATION_PLAN.md.
//
// maxTokens is fixed at 4096 for every provider here, matching what the
// Claude-only path used before this migration (internal/integrations/claude.
// Client.Analyze) — a deliberate increase for Groq, which previously
// hard-capped Analyze-style calls at 512 tokens (groq.Client.GenerateReply's
// fixed setting) and Gemini, which previously had no explicit cap at all.
// CRM-doc parsing can produce a sizeable structured JSON mapping, so 4096
// uniformly is a correctness improvement, not just a migration side effect.
const analyzeMaxTokens = 4096

type bifrostAdapter struct {
	client    *bifrost.Client
	provider  bifrost.ProviderName
	model     string
	directKey string
}

func (a *bifrostAdapter) Analyze(ctx context.Context, systemPrompt, document string) (Reply, error) {
	r, err := a.client.Complete(ctx, bifrost.Request{
		Provider:     a.provider,
		Model:        a.model,
		DirectKey:    a.directKey,
		SystemPrompt: systemPrompt,
		Prompt:       document,
		MaxTokens:    analyzeMaxTokens,
	})
	return Reply{Text: r.Text, TokensIn: r.TokensIn, TokensOut: r.TokensOut}, err
}
