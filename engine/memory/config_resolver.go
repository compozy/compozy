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
	// TODO: Implement proper config loading once registry interface is stable
	return nil, memcore.NewConfigError(
		fmt.Sprintf("memory resource '%s' not implemented yet", resourceID),
		fmt.Errorf("config loading not implemented"),
	)
}

// resolveMemoryKey evaluates the memory key template and returns the resolved key
func (mm *Manager) resolveMemoryKey(
	_ context.Context,
	keyTemplate string,
	workflowContextData map[string]any,
) (string, string) {
	// TODO: Implement template evaluation once template engine interface is stable
	sanitizedKey := mm.sanitizeKey(keyTemplate)
	projectIDVal := ""
	if projectID, ok := workflowContextData["project.id"]; ok {
		if projectIDStr, ok := projectID.(string); ok {
			projectIDVal = projectIDStr
		}
	}
	return sanitizedKey, projectIDVal
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
