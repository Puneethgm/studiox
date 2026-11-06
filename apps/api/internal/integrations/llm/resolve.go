package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/projectx/api/internal/integrations/bifrost"
)

// PlatformSettings is the studios.Repo method Resolver needs to read the
// centrally-configured, superadmin-managed Groq/Gemini/Claude keys (see
// studios.Repo.GetPlatformAIProviderKey). Declared as an interface here, not
// imported from internal/studios directly, because internal/studios depends
// on this package's sibling internal/integrations/crm (for crm.Executor) —
// importing the concrete *studios.Repo type here would create an import
// cycle (llm -> studios -> crm -> llm).
type PlatformSettings interface {
	GetPlatformAIProviderKey(ctx context.Context, provider string) (string, error)
}

// providerBifrostName maps this package's own ProviderName (which matches
// ai_task_configs.provider / studio_ai_models.provider values, "claude")
// to Bifrost's provider id for the same service, which is "anthropic" —
// confirmed against a real running Bifrost instance (see
// BIFROST_MIGRATION_PLAN.md Phase 0). Nothing in ai_task_configs or
// studio_ai_models needs to change; this mapping is purely an
// implementation detail of talking to Bifrost.
var providerBifrostName = map[ProviderName]bifrost.ProviderName{
	ProviderClaude: bifrost.ProviderAnthropic,
	ProviderGemini: bifrost.ProviderGemini,
	ProviderGroq:   bifrost.ProviderGroq,
	ProviderOpenAI: bifrost.ProviderOpenAI,
}

// Resolver turns "which LLM handles purpose X" into a callable Provider. It
// holds the platform's Bifrost client (shared, one instance for the whole
// app) plus PlatformSettings to resolve the right direct key per request —
// the same centrally-configured, superadmin-managed Groq/Gemini/Claude key
// every other AI call site in the app now uses (see
// BIFROST_MIGRATION_PLAN.md and studios.Repo.GetPlatformAIProviderKey).
type Resolver struct {
	repo *Repo

	bifrostClient    *bifrost.Client
	platformSettings PlatformSettings
}

func NewResolver(repo *Repo, bifrostClient *bifrost.Client, platformSettings PlatformSettings) *Resolver {
	return &Resolver{
		repo:             repo,
		bifrostClient:    bifrostClient,
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

	bifrostProvider, ok := providerBifrostName[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q for %q", cfg.Provider, purpose)
	}

	directKey := overrideKey
	switch cfg.Provider {
	case ProviderClaude, ProviderGemini, ProviderGroq, ProviderOpenAI:
		if directKey == "" {
			directKey, _ = r.platformSettings.GetPlatformAIProviderKey(ctx, string(cfg.Provider))
		}
		if directKey == "" {
			return nil, fmt.Errorf("%s not configured (no platform key and no override for %q)", cfg.Provider, purpose)
		}
	}

	if r.bifrostClient == nil {
		return nil, fmt.Errorf("bifrost gateway not configured")
	}

	return &bifrostAdapter{
		client:    r.bifrostClient,
		provider:  bifrostProvider,
		model:     cfg.Model,
		directKey: directKey,
	}, nil
}
