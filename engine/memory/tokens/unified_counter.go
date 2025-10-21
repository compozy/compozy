package tokens

import (
	"context"
	"fmt"
	"sync"
	"time"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/open-and-sustainable/alembica/llm/tokens"
)

const (
	// DefaultAPITimeout is the default timeout for token counting API calls
	DefaultAPITimeout = 5 * time.Second
)

// UnifiedTokenCounter provides multi-provider token counting with fallback to tiktoken
type UnifiedTokenCounter struct {
	realCounter     *tokens.RealTokenCounter
	fallbackCounter memcore.TokenCounter // tiktoken fallback
	providerConfig  *ProviderConfig
	mu              sync.RWMutex
}

// ProviderConfig holds configuration for a specific provider
type ProviderConfig struct {
	Provider string            // "openai", "anthropic", "google", etc.
	Model    string            // Model name
	APIKey   string            // API key for real-time counting
	Endpoint string            // Optional custom endpoint
	Settings map[string]string // Provider-specific settings
}

// NewUnifiedTokenCounter creates a new unified token counter
func NewUnifiedTokenCounter(
	providerConfig *ProviderConfig,
	fallbackCounter memcore.TokenCounter,
) (*UnifiedTokenCounter, error) {
	if fallbackCounter == nil {
		return nil, fmt.Errorf("fallback counter cannot be nil")
	}
	realCounter := &tokens.RealTokenCounter{}
	return &UnifiedTokenCounter{
		realCounter:     realCounter,
		fallbackCounter: fallbackCounter,
		providerConfig:  providerConfig,
	}, nil
}

// CountTokens counts tokens using the configured provider or falls back to tiktoken
func (u *UnifiedTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	config := u.GetProviderConfig()
	if !isProviderConfigured(config) {
		return u.countWithFallback(ctx, text)
	}
	apiCtx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()
	count, err := u.tryCountWithProvider(apiCtx, config, text)
	if err != nil {
		return u.handleProviderFailure(ctx, text, config, err)
	}
	if count == 0 {
		return u.handleZeroCount(ctx, text, config)
	}
	return count, nil
}

func isProviderConfigured(config *ProviderConfig) bool {
	return config != nil && config.APIKey != ""
}

func (u *UnifiedTokenCounter) tryCountWithProvider(
	ctx context.Context,
	config *ProviderConfig,
	text string,
) (int, error) {
	resultChan := make(chan int, 1)
	go func() {
		count := u.realCounter.GetNumTokensFromPrompt(
			text,
			config.Provider,
			config.Model,
			config.APIKey,
		)
		select {
		case resultChan <- count:
		case <-ctx.Done():
		}
	}()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case count := <-resultChan:
		return count, nil
	}
}

func (u *UnifiedTokenCounter) handleProviderFailure(
	ctx context.Context,
	text string,
	config *ProviderConfig,
	cause error,
) (int, error) {
	log := logger.FromContext(ctx)
	log.Warn("Token counting API call timed out, using fallback",
		"provider", config.Provider,
		"model", config.Model,
		"timeout", DefaultAPITimeout,
		"error", cause,
	)
	return u.countWithFallback(ctx, text)
}

func (u *UnifiedTokenCounter) handleZeroCount(
	ctx context.Context,
	text string,
	config *ProviderConfig,
) (int, error) {
	log := logger.FromContext(ctx)
	log.Warn("Real-time token counting failed or returned zero, using fallback",
		"provider", config.Provider,
		"model", config.Model,
	)
	return u.countWithFallback(ctx, text)
}

func (u *UnifiedTokenCounter) countWithFallback(ctx context.Context, text string) (int, error) {
	return u.fallbackCounter.CountTokens(ctx, text)
}

// GetEncoding returns a string identifying the encoding/provider being used
func (u *UnifiedTokenCounter) GetEncoding() string {
	u.mu.RLock()
	config := u.providerConfig
	u.mu.RUnlock()
	if config != nil {
		return fmt.Sprintf("%s-%s", config.Provider, config.Model)
	}
	return u.fallbackCounter.GetEncoding()
}

// UpdateProvider updates the provider configuration
func (u *UnifiedTokenCounter) UpdateProvider(config *ProviderConfig) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.providerConfig = config
}

// GetProviderConfig returns the current provider configuration
func (u *UnifiedTokenCounter) GetProviderConfig() *ProviderConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.providerConfig
}

// IsFallbackActive returns true if the counter is currently using fallback
func (u *UnifiedTokenCounter) IsFallbackActive() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.providerConfig == nil || u.providerConfig.APIKey == ""
}
