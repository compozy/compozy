package llmadapter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// DefaultFactory is the default implementation of Factory
type DefaultFactory struct{}

// NewDefaultFactory creates a new default factory
func NewDefaultFactory() Factory {
	return &DefaultFactory{}
}

// CreateClient creates a new LLMClient for the given provider
func (f *DefaultFactory) CreateClient(config *core.ProviderConfig) (LLMClient, error) {
	switch config.Provider {
	case core.ProviderOpenAI, core.ProviderGroq, core.ProviderOllama:
		return NewLangChainAdapter(config)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}
