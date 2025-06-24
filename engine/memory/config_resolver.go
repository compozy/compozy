package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	memcore "github.com/compozy/compozy/engine/memory/core"
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
	return &memcore.Resource{
		ID:               config.ID,
		Description:      config.Description,
		Type:             config.Type,
		Model:            "", // Not directly mapped from config
		ModelContextSize: 0,  // Not directly mapped from config
		MaxTokens:        config.MaxTokens,
		MaxMessages:      config.MaxMessages,
		MaxContextRatio:  config.MaxContextRatio,
		EvictionPolicy:   "", // Not directly mapped from config
		TokenAllocation:  config.TokenAllocation,
		FlushingStrategy: config.Flushing,
		Persistence:      config.Persistence,
		AppendTTL:        "", // Not directly mapped from config
		ClearTTL:         "", // Not directly mapped from config
		FlushTTL:         "", // Not directly mapped from config
		PrivacyPolicy:    config.PrivacyPolicy,
		TokenCounter:     "", // Not directly mapped from config
		Metadata:         nil,
		DisableFlush:     false, // Not directly mapped from config
	}
}

// resolveMemoryKey evaluates the memory key template and returns the resolved key
func (mm *Manager) resolveMemoryKey(
	_ context.Context,
	keyTemplate string,
	workflowContextData map[string]any,
) (string, string) {
	// Use existing pkg/tplengine - no new dependencies needed
	result, err := mm.tplEngine.ProcessString(keyTemplate, workflowContextData)
	if err != nil {
		// Fall back to sanitizing the template as-is with warning
		mm.log.Warn("Failed to evaluate key template",
			"template", keyTemplate,
			"error", err)
		sanitizedKey := mm.sanitizeKey(keyTemplate)
		projectIDVal := extractProjectID(workflowContextData)
		return sanitizedKey, projectIDVal
	}
	// Sanitize the resolved key and extract project ID
	resolvedKey := result.Text
	sanitizedKey := mm.sanitizeKey(resolvedKey)
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
	// Try to get from cache first
	if mm.componentCache != nil {
		cacheKey := fmt.Sprintf("token-counter:%s", model)
		if cached, found := mm.componentCache.Get(cacheKey); found {
			if counter, ok := cached.(*CacheableTiktokenCounter); ok {
				return counter.TiktokenCounter, nil
			}
		}
	}
	// Create new token counter
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
