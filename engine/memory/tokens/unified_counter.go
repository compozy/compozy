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
	log             logger.Logger
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
		log:             log,
	}, nil
}

// CountTokens counts tokens using the configured provider or falls back to tiktoken
func (u *UnifiedTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	u.mu.RLock()
	config := u.providerConfig
	u.mu.RUnlock()
	if config == nil || config.APIKey == "" {
		// No real-time counting configured, use fallback
		return u.fallbackCounter.CountTokens(ctx, text)
	}
	// Create a context with timeout for external API call
	apiCtx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()
	// Channel to receive the result
	type result struct {
		count int
		err   error
	}
	resultChan := make(chan result, 1)
	// Run the API call in a goroutine to respect the timeout
	go func() {
		// Try real-time API counting
		count := u.realCounter.GetNumTokensFromPrompt(
			text,
			config.Provider,
			config.Model,
			config.APIKey,
		)
		// Use select to prevent blocking if context is already done
		select {
		case resultChan <- result{count: count, err: nil}:
			// Result sent successfully
		case <-apiCtx.Done():
			// Context was canceled, abort sending and exit goroutine
			return
		}
	}()
	// Wait for result or timeout
	select {
	case <-apiCtx.Done():
		// Timeout occurred
		if u.log != nil {
			u.log.Warn("Token counting API call timed out, using fallback",
				"provider", config.Provider,
				"model", config.Model,
				"timeout", DefaultAPITimeout)
		}
		return u.fallbackCounter.CountTokens(ctx, text)
	case res := <-resultChan:
		if res.count == 0 {
			// Alembica returns 0 for errors or unsupported providers
			if u.log != nil {
				u.log.Warn("Real-time token counting failed or returned zero, using fallback",
					"provider", config.Provider,
					"model", config.Model)
			}
			// Fallback to tiktoken
			return u.fallbackCounter.CountTokens(ctx, text)
		}
		return res.count, nil
	}
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
