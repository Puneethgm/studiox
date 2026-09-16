package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/projectx/api/internal/integrations/claude"
	"github.com/projectx/api/internal/integrations/gemini"
	"github.com/projectx/api/internal/integrations/groq"
)

// PlatformSettings is the one studios.Repo/studios.Service method Resolver
// needs (Gemini's platform-level API key, which — unlike Claude/Groq — has
// no fixed .env var). Declared as an interface here, not imported from
// internal/studios directly, because internal/studios depends on this
// package's sibling internal/integrations/crm (for crm.Executor) — importing
// the concrete *studios.Repo type here would create an import cycle
// (llm -> studios -> crm -> llm).
type PlatformSettings interface {
	GetPlatformSetting(ctx context.Context, key string) (string, error)
}

// Resolver turns "which LLM handles purpose X" into a callable Provider. It
// holds the platform's already-constructed Claude/Groq clients (so the
// common case — no per-task key override — reuses the exact same client
// instance the rest of the app uses, no re-reading env vars) plus enough to
// build a one-off client when a task's ai_task_configs row overrides the key.
type Resolver struct {
	repo *Repo

	claudeClient     *claude.Client
	claudeAPIURL     string
	groqClient       *groq.Client
	geminiClient     *gemini.Client
	platformSettings PlatformSettings
}

func NewResolver(repo *Repo, claudeClient *claude.Client, claudeAPIURL string, groqClient *groq.Client, platformSettings PlatformSettings) *Resolver {
	return &Resolver{
		repo:             repo,
		claudeClient:     claudeClient,
		claudeAPIURL:     claudeAPIURL,
		groqClient:       groqClient,
		geminiClient:     gemini.New(),
		platformSettings: platformSettings,
	}
}

// ResolveProvider reads ai_task_configs for purpose and returns the
// configured provider. There's no silent default — if a super-admin hasn't
// picked a provider for this task yet, the caller gets a clear error
// telling them to configure it, rather than an arbitrary guess.
func (r *Resolver) ResolveProvider(ctx context.Context, purpose string) (Provider, error) {
	cfg, err := r.repo.GetTaskConfig(ctx, purpose)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("no AI provider configured for %q — set one in Settings first", purpose)
		}
		return nil, err
	}
	overrideKey, err := r.repo.DecryptAPIKey(cfg)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key override: %w", err)
	}

	switch cfg.Provider {
	case ProviderClaude:
		client := r.claudeClient
		if overrideKey != "" {
			client, err = claude.New(r.claudeAPIURL, overrideKey)
			if err != nil {
				return nil, fmt.Errorf("build claude client: %w", err)
			}
		}
		if client == nil {
			return nil, fmt.Errorf("claude not configured (no platform key and no override for %q)", purpose)
		}
		return &claudeAdapter{client: client, model: cfg.Model}, nil

	case ProviderGemini:
		key := overrideKey
		if key == "" {
			key, _ = r.platformSettings.GetPlatformSetting(ctx, "gemini_api_key")
		}
		if key == "" {
			return nil, fmt.Errorf("gemini not configured (no platform key and no override for %q)", purpose)
		}
		return &geminiAdapter{client: r.geminiClient, apiKey: key}, nil

	case ProviderGroq:
		client := r.groqClient
		if overrideKey != "" {
			client = groq.New(overrideKey)
		}
		if client == nil {
			return nil, fmt.Errorf("groq not configured (no platform key and no override for %q)", purpose)
		}
		return &groqAdapter{client: client, model: cfg.Model}, nil

	default:
		return nil, fmt.Errorf("unsupported provider %q for %q", cfg.Provider, purpose)
	}
}
