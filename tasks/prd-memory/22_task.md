---
status: pending
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>multi_provider_tokens</dependencies>
</task_context>

# Task 22.0: Implement Multi-Provider Token Counting

## Overview

Replace the current tiktoken-only token counting with a unified multi-provider token counter using `github.com/open-and-sustainable/alembica/llm/tokens`. This enables accurate token counting for OpenAI, GoogleAI, Cohere, Anthropic, and DeepSeek models with real-time API-based counting and intelligent fallbacks.

## Subtasks

- [ ] 22.1 Add alembica library dependency to go.mod
- [ ] 22.2 Implement unified token counter with multi-provider support
- [ ] 22.3 Add provider registry and configuration system
- [ ] 22.4 Implement intelligent fallback to tiktoken when API unavailable
- [ ] 22.5 Add comprehensive tests for all provider types
- [ ] 22.6 Update memory configuration to support provider selection

## Implementation Details

### Library Dependencies

Add to `go.mod`:

```go
require (
    github.com/open-and-sustainable/alembica v1.0.0    // Multi-provider token counting
)
```

### Unified Token Counter Implementation

```go
// engine/memory/tokens/unified_counter.go
package tokens

import (
    "context"
    "fmt"
    "sync"

    "github.com/open-and-sustainable/alembica/llm/tokens"
    "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/pkg/logger"
)

type UnifiedTokenCounter struct {
    realCounter    *tokens.RealTokenCounter
    fallbackCounter core.TokenCounter // tiktoken fallback
    providerConfig *ProviderConfig
    log           logger.Logger
    mu            sync.RWMutex
}

type ProviderConfig struct {
    Provider string            // "openai", "anthropic", "google", etc.
    Model    string            // Model name
    APIKey   string            // API key for real-time counting
    Endpoint string            // Optional custom endpoint
    Settings map[string]string // Provider-specific settings
}

func NewUnifiedTokenCounter(
    providerConfig *ProviderConfig,
    fallbackCounter core.TokenCounter,
    log logger.Logger,
) (*UnifiedTokenCounter, error) {

    if fallbackCounter == nil {
        return nil, fmt.Errorf("fallback counter cannot be nil")
    }

    realCounter := &tokens.RealTokenCounter{}

    return &UnifiedTokenCounter{
        realCounter:     realCounter,
        fallbackCounter: fallbackCounter,
        providerConfig:  providerConfig,
        log:            log,
    }, nil
}

func (u *UnifiedTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
    u.mu.RLock()
    config := u.providerConfig
    u.mu.RUnlock()

    if config == nil || config.APIKey == "" {
        // No real-time counting configured, use fallback
        return u.fallbackCounter.CountTokens(ctx, text)
    }

    // Try real-time API counting first
    count, err := u.realCounter.GetNumTokensFromPrompt(
        text,
        config.Provider,
        config.Model,
        config.APIKey,
    )

    if err != nil {
        u.log.Warn("Real-time token counting failed, using fallback",
            "provider", config.Provider,
            "model", config.Model,
            "error", err)

        // Fallback to tiktoken
        return u.fallbackCounter.CountTokens(ctx, text)
    }

    return count, nil
}

func (u *UnifiedTokenCounter) GetEncoding() string {
    u.mu.RLock()
    config := u.providerConfig
    u.mu.RUnlock()

    if config != nil {
        return fmt.Sprintf("%s-%s", config.Provider, config.Model)
    }

    return u.fallbackCounter.GetEncoding()
}

func (u *UnifiedTokenCounter) UpdateProvider(config *ProviderConfig) {
    u.mu.Lock()
    defer u.mu.Unlock()
    u.providerConfig = config
}
```

### Provider Registry

```go
// engine/memory/tokens/provider_registry.go
package tokens

import (
    "fmt"
    "sync"
)

type ProviderRegistry struct {
    providers map[string]*ProviderConfig
    mu       sync.RWMutex
}

func NewProviderRegistry() *ProviderRegistry {
    return &ProviderRegistry{
        providers: make(map[string]*ProviderConfig),
    }
}

func (r *ProviderRegistry) Register(name string, config *ProviderConfig) error {
    if name == "" {
        return fmt.Errorf("provider name cannot be empty")
    }

    if config == nil {
        return fmt.Errorf("provider config cannot be nil")
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    r.providers[name] = config
    return nil
}

func (r *ProviderRegistry) Get(name string) (*ProviderConfig, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    config, exists := r.providers[name]
    if !exists {
        return nil, fmt.Errorf("provider '%s' not found", name)
    }

    return config, nil
}

func (r *ProviderRegistry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    providers := make([]string, 0, len(r.providers))
    for name := range r.providers {
        providers = append(providers, name)
    }

    return providers
}

// Pre-configured provider configs
func (r *ProviderRegistry) RegisterDefaults() {
    r.Register("openai-gpt4", &ProviderConfig{
        Provider: "openai",
        Model:    "gpt-4",
        Settings: map[string]string{
            "encoding": "cl100k_base",
        },
    })

    r.Register("anthropic-claude", &ProviderConfig{
        Provider: "anthropic",
        Model:    "claude-3-haiku",
        Settings: map[string]string{
            "encoding": "claude",
        },
    })

    r.Register("google-gemini", &ProviderConfig{
        Provider: "google",
        Model:    "gemini-pro",
        Settings: map[string]string{
            "encoding": "gemini",
        },
    })
}
```

### Memory Configuration Updates

```go
// Update engine/memory/core/types.go to include provider config
type Resource struct {
    // ... existing fields ...

    // Token counting configuration
    TokenProvider *TokenProviderConfig `yaml:"token_provider,omitempty" json:"token_provider,omitempty"`
}

type TokenProviderConfig struct {
    Provider string            `yaml:"provider" json:"provider"`                   // "openai", "anthropic", etc.
    Model    string            `yaml:"model" json:"model"`                         // Model name
    APIKey   string            `yaml:"api_key,omitempty" json:"api_key,omitempty"` // API key for real-time counting
    Fallback string            `yaml:"fallback" json:"fallback"`                   // Fallback strategy
    Settings map[string]string `yaml:"settings,omitempty" json:"settings,omitempty"`
}
```

### Token Counter Factory

```go
// engine/memory/tokens/factory.go
package tokens

import (
    "fmt"

    "github.com/compozy/compozy/engine/memory/core"
    "github.com/compozy/compozy/pkg/logger"
)

type CounterFactory struct {
    registry         *ProviderRegistry
    fallbackFactory  func() (core.TokenCounter, error)
}

func NewCounterFactory(fallbackFactory func() (core.TokenCounter, error)) *CounterFactory {
    return &CounterFactory{
        registry:        NewProviderRegistry(),
        fallbackFactory: fallbackFactory,
    }
}

func (f *CounterFactory) CreateCounter(
    config *core.TokenProviderConfig,
    log logger.Logger,
) (core.TokenCounter, error) {

    // Create fallback counter
    fallback, err := f.fallbackFactory()
    if err != nil {
        return nil, fmt.Errorf("failed to create fallback counter: %w", err)
    }

    if config == nil {
        // No provider config, use fallback only
        return fallback, nil
    }

    // Create provider config
    providerConfig := &ProviderConfig{
        Provider: config.Provider,
        Model:    config.Model,
        APIKey:   config.APIKey,
        Settings: config.Settings,
    }

    // Create unified counter
    return NewUnifiedTokenCounter(providerConfig, fallback, log)
}
```

**Key Implementation Notes:**

- Eliminates provider-specific token counting implementations (~200 lines)
- Real-time API-based counting for accurate results
- Intelligent fallback to tiktoken when API unavailable
- Provider registry for centralized configuration
- Configuration-driven provider selection
- Maintains backward compatibility with existing tiktoken usage

## Success Criteria

- ✅ Multi-provider token counting supports OpenAI, Anthropic, Google, Cohere, DeepSeek
- ✅ Real-time API-based counting provides accurate token counts
- ✅ Intelligent fallback to tiktoken when API unavailable or fails
- ✅ Provider registry enables centralized configuration management
- ✅ Memory configuration supports provider selection
- ✅ Comprehensive tests validate all provider types and fallback scenarios
- ✅ Performance is acceptable for real-time counting operations
- ✅ Integration with existing memory system is seamless

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** use `github.com/open-and-sustainable/alembica/llm/tokens` for multi-provider support
- **MUST** maintain backward compatibility with existing tiktoken usage
- **MUST** implement robust fallback mechanism for API failures
- **MUST** include comprehensive test coverage for all providers
- **MUST** follow established configuration patterns
- **MUST** add proper error handling and logging
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
