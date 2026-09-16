// Package llm resolves "which LLM should do task X" to a concrete callable
// provider, so a feature like CRM-doc parsing (internal/integrations/crm)
// isn't hardcoded to one of the platform's three LLM clients
// (claude/gemini/groq). A super-admin picks the provider + model per task
// (ai_task_configs); this package reads that config and hands back a
// Provider satisfying a single common interface.
package llm

import (
	"time"
)

type ProviderName string

const (
	ProviderClaude ProviderName = "claude"
	ProviderGemini ProviderName = "gemini"
	ProviderGroq   ProviderName = "groq"
)

// TaskConfig is one row of ai_task_configs — which provider/model handles
// one named AI-driven task. APIKeyEnc nil/empty means "use that provider's
// existing platform-level key" rather than requiring every task to have its
// own duplicate key.
type TaskConfig struct {
	Purpose   string
	Provider  ProviderName
	Model     string
	APIKeyEnc string
	UpdatedAt time.Time
}
