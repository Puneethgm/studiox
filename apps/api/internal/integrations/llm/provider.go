package llm

import (
	"context"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/gemini"
	"github.com/projectx/api/internal/integrations/groq"
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

type claudeAdapter struct {
	client *claude.Client
	model  string
}

func (a *claudeAdapter) Analyze(ctx context.Context, systemPrompt, document string) (Reply, error) {
	r, err := a.client.Analyze(ctx, systemPrompt, document, a.model)
	return Reply{Text: r.Text, TokensIn: r.TokensIn, TokensOut: r.TokensOut}, err
}

type geminiAdapter struct {
	client *gemini.Client
	apiKey string
}

func (a *geminiAdapter) Analyze(ctx context.Context, systemPrompt, document string) (Reply, error) {
	// Gemini's client has no system-prompt param — folded into one prompt,
	// same accommodation ai_worker.go already makes for Gemini elsewhere.
	prompt := document
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + document
	}
	r, err := a.client.GenerateReply(ctx, a.apiKey, prompt)
	return Reply{Text: r.Text, TokensIn: r.TokensIn, TokensOut: r.TokensOut}, err
}

type groqAdapter struct {
	client *groq.Client
	model  string
}

func (a *groqAdapter) Analyze(ctx context.Context, systemPrompt, document string) (Reply, error) {
	// Same accommodation as Gemini — groq.Client.GenerateReply takes one
	// flat prompt, no separate system-prompt param.
	prompt := document
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + document
	}
	r, err := a.client.GenerateReply(ctx, prompt, a.model)
	return Reply{Text: r.Text, TokensIn: r.TokensIn, TokensOut: r.TokensOut}, err
}
