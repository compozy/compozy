package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/tokens"
)

// loadMemoryConfig loads and validates a memory configuration by ID
func (mm *Manager) loadMemoryConfig(resourceID string) (*memcore.Resource, error) {
	// Use existing autoload.ConfigRegistry - no new dependencies needed
	config, err := mm.resourceRegistry.Get("memory", resourceID)
	if err != nil {
		return nil, memcore.NewConfigError(
			fmt.Sprintf("memory resource '%s' not found in registry", resourceID),
			err,
		)
	}
	// Type assert to expected memory config type
	memConfig, ok := config.(*Config)
	if !ok {
		return nil, memcore.NewConfigError(
			fmt.Sprintf("invalid config type for memory resource '%s'", resourceID),
			fmt.Errorf("expected *memory.Config, got %T", config),
		)
	}
	// Convert Config to memcore.Resource
	return mm.configToResource(memConfig), nil
}

// configToResource converts a memory Config to a memcore.Resource
func (mm *Manager) configToResource(config *Config) *memcore.Resource {
	resource := &memcore.Resource{
		ID:               config.ID,
		Description:      config.Description,
		Type:             config.Type,
		Model:            "", // Model not specified in memory config
		ModelContextSize: 0,  // Model context size not specified in memory config
		MaxTokens:        config.MaxTokens,
		MaxMessages:      config.MaxMessages,
		MaxContextRatio:  config.MaxContextRatio,
		EvictionPolicy:   "", // Eviction policy determined by memory type and flushing strategy
		TokenAllocation:  config.TokenAllocation,
		FlushingStrategy: config.Flushing,
		Persistence:      config.Persistence,
		PrivacyPolicy:    config.PrivacyPolicy,
		TokenCounter:     "",    // Token counter determined at runtime
		Metadata:         nil,   // Metadata not stored in config
		DisableFlush:     false, // Flush enabled by default
	}
	// Map TTL fields from locking configuration if present
	if config.Locking != nil {
		resource.AppendTTL = config.Locking.AppendTTL
		resource.ClearTTL = config.Locking.ClearTTL
		resource.FlushTTL = config.Locking.FlushTTL
	}
	// Set the parsed TTL from persistence config
	resource.Persistence.ParsedTTL = config.Persistence.ParsedTTL
	// Debug log
	mm.log.Debug("Config to resource conversion",
		"config_ttl", config.Persistence.TTL,
		"parsed_ttl", config.Persistence.ParsedTTL,
		"resource_id", resource.ID)
	return resource
}

// resolveMemoryKey evaluates the memory key template and returns the resolved key
func (mm *Manager) resolveMemoryKey(
	_ context.Context,
	keyTemplate string,
	workflowContextData map[string]any,
) (string, string) {
	var keyToSanitize string
	// Use existing pkg/tplengine - no new dependencies needed
	result, err := mm.tplEngine.ProcessString(keyTemplate, workflowContextData)
	if err != nil {
		// Fall back to sanitizing the template as-is with warning
		mm.log.Warn("Failed to evaluate key template",
			"template", keyTemplate,
			"error", err)
		keyToSanitize = keyTemplate
	} else {
		keyToSanitize = result.Text
	}
	// Sanitize the key (either resolved or fallback) and extract project ID
	sanitizedKey := mm.sanitizeKey(keyToSanitize)
	projectIDVal := extractProjectID(workflowContextData)
	return sanitizedKey, projectIDVal
}

// extractProjectID extracts project ID from workflow context data
func extractProjectID(workflowContextData map[string]any) string {
	if projectID, ok := workflowContextData["project.id"]; ok {
		if projectIDStr, ok := projectID.(string); ok {
			return projectIDStr
		}
	}
	return ""
}

// sanitizeKey creates a safe, deterministic key for Redis storage
func (mm *Manager) sanitizeKey(key string) string {
	// Use SHA-256 hash for consistent, safe keys
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// registerPrivacyPolicy registers the privacy policy if one is configured
func (mm *Manager) registerPrivacyPolicy(resourceCfg *memcore.Resource) error {
	if resourceCfg.PrivacyPolicy != nil {
		if err := mm.privacyManager.RegisterPolicy(resourceCfg.ID, resourceCfg.PrivacyPolicy); err != nil {
			mm.log.Error("Failed to register privacy policy", "resource_id", resourceCfg.ID, "error", err)
			return fmt.Errorf("failed to register privacy policy for resource '%s': %w", resourceCfg.ID, err)
		}
	}
	return nil
}

// getOrCreateTokenCounter retrieves or creates a token counter for the given model
func (mm *Manager) getOrCreateTokenCounter(model string) (memcore.TokenCounter, error) {
	return mm.getOrCreateTokenCounterWithConfig(model, nil)
}

// getOrCreateTokenCounterWithConfig retrieves or creates a token counter for
// the given model and optional provider config
func (mm *Manager) getOrCreateTokenCounterWithConfig(
	model string,
	providerConfig *memcore.TokenProviderConfig,
) (memcore.TokenCounter, error) {
	// Determine cache key based on configuration
	var cacheKey string
	if providerConfig != nil {
		// Include provider info in cache key for unified counters
		keyHash := ""
		if providerConfig.APIKey != "" {
			// Use SHA-256 hash of the API key to guarantee uniqueness
			hasher := sha256.New()
			hasher.Write([]byte(providerConfig.APIKey))
			// Use first 16 characters of hex hash for cache key (sufficient for uniqueness)
			keyHash = ":" + hex.EncodeToString(hasher.Sum(nil))[:16]
		}
		cacheKey = fmt.Sprintf("unified-counter:%s:%s%s", providerConfig.Provider, providerConfig.Model, keyHash)
	} else {
		cacheKey = fmt.Sprintf("token-counter:%s", model)
	}
	// Try to get from cache first
	if mm.componentCache != nil {
		if cached, found := mm.componentCache.Get(cacheKey); found {
			if providerConfig != nil {
				if counter, ok := cached.(*CacheableUnifiedCounter); ok {
					return counter.UnifiedTokenCounter, nil
				}
			} else {
				if counter, ok := cached.(*CacheableTiktokenCounter); ok {
					return counter.TiktokenCounter, nil
				}
			}
		}
	}
	// Create new token counter based on configuration
	if providerConfig != nil {
		// Create API key resolver
		keyResolver := tokens.NewAPIKeyResolver(mm.log)
		// Resolve provider configuration with API key from environment
		tokensProviderConfig := keyResolver.ResolveProviderConfig(providerConfig)
		cacheableCounter, err := NewCacheableUnifiedCounter(tokensProviderConfig, model)
		if err != nil {
			return nil, fmt.Errorf("failed to create unified counter: %w", err)
		}
		// Store in cache if available
		if mm.componentCache != nil {
			mm.componentCache.Set(cacheableCounter.GetCacheKey(), cacheableCounter)
		}
		return cacheableCounter.UnifiedTokenCounter, nil
	}
	// Create traditional tiktoken counter
	cacheableCounter, err := NewCacheableTiktokenCounter(model)
	if err != nil {
		return nil, err
	}
	// Store in cache if available
	if mm.componentCache != nil {
		mm.componentCache.Set(cacheableCounter.GetCacheKey(), cacheableCounter)
	}
	return cacheableCounter.TiktokenCounter, nil
}
