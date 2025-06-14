package llmadapter

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// DefaultFactory is a default implementation of the Factory interface
type DefaultFactory struct{}

// NewDefaultFactory creates a new DefaultFactory
func NewDefaultFactory() Factory {
	return &DefaultFactory{}
}

// CreateClient creates a new LLMClient for the given provider
func (f *DefaultFactory) CreateClient(config *core.ProviderConfig) (LLMClient, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	switch config.Provider {
	case core.ProviderOpenAI, core.ProviderAnthropic, core.ProviderGroq,
		core.ProviderMock, core.ProviderOllama, core.ProviderGoogle,
		core.ProviderDeepSeek, core.ProviderXAI:
		return NewLangChainAdapter(config)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}
