package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/memory/tokens"
	"github.com/compozy/compozy/pkg/logger"
)

// ErrorType represents different types of memory errors
type ErrorType string

const (
	ErrorTypeConfig ErrorType = "configuration"
	ErrorTypeLock   ErrorType = "lock"
	ErrorTypeStore  ErrorType = "store"
	ErrorTypeCache  ErrorType = "cache"
)

// Error provides structured error information
type Error struct {
	Type       ErrorType
	Operation  string
	ResourceID string
	Cause      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("memory %s error for resource '%s' during %s: %v",
		e.Type, e.ResourceID, e.Operation, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// ResourceBuilder provides a clean way to construct memcore.Resource from Config
type ResourceBuilder struct {
	config *Config
	logger logger.Logger
}

// Build constructs a memcore.Resource with all necessary mappings
func (rb *ResourceBuilder) Build() (*memcore.Resource, error) {
	resource := &memcore.Resource{
		ID:                   rb.config.ID,
		Description:          rb.config.Description,
		Type:                 rb.config.Type,
		Model:                "", // Model not specified in memory config
		ModelContextSize:     0,  // Model context size not specified in memory config
		MaxTokens:            rb.config.MaxTokens,
		MaxMessages:          rb.config.MaxMessages,
		MaxContextRatio:      rb.config.MaxContextRatio,
		EvictionPolicyConfig: nil, // Eviction policy determined by memory type and flushing strategy
		TokenAllocation:      rb.config.TokenAllocation,
		FlushingStrategy:     rb.config.Flushing,
		Persistence:          rb.config.Persistence,
		TokenCounter:         "", // Token counter determined at runtime
		TokenProvider:        rb.config.TokenProvider,
		Metadata:             nil,   // Metadata not stored in config
		DisableFlush:         false, // Flush enabled by default
	}
	// Apply privacy policy if configured
	if err := rb.applyPrivacyPolicy(resource); err != nil {
		return nil, err
	}
	// Apply locking configuration
	rb.applyLockingConfig(resource)
	// Apply persistence configuration
	rb.applyPersistenceConfig(resource)
	// Log the conversion
	rb.logger.Debug("Config to resource conversion",
		"config_ttl", rb.config.Persistence.TTL,
		"parsed_ttl", rb.config.Persistence.ParsedTTL,
		"resource_id", resource.ID)
	return resource, nil
}

func (rb *ResourceBuilder) applyPrivacyPolicy(resource *memcore.Resource) error {
	if rb.config.PrivacyPolicy == nil {
		return nil
	}
	resource.PrivacyPolicy = &memcore.PrivacyPolicyConfig{
		NonPersistableMessageTypes: rb.config.PrivacyPolicy.NonPersistableMessageTypes,
		DefaultRedactionString:     rb.config.PrivacyPolicy.DefaultRedactionString,
	}
	// Validate and use regex patterns directly
	if len(rb.config.PrivacyPolicy.RedactPatterns) > 0 {
		if err := privacy.ValidateRedactionPatterns(rb.config.PrivacyPolicy.RedactPatterns); err != nil {
			return &Error{
				Type:       ErrorTypeConfig,
				Operation:  "privacy_policy_validation",
				ResourceID: rb.config.ID,
				Cause:      err,
			}
		}
		resource.PrivacyPolicy.RedactPatterns = rb.config.PrivacyPolicy.RedactPatterns
		rb.logger.Debug("Using redaction patterns",
			"patterns", rb.config.PrivacyPolicy.RedactPatterns)
	}
	return nil
}

func (rb *ResourceBuilder) applyLockingConfig(resource *memcore.Resource) {
	if rb.config.Locking == nil {
		return
	}
	resource.AppendTTL = rb.config.Locking.AppendTTL
	resource.ClearTTL = rb.config.Locking.ClearTTL
	resource.FlushTTL = rb.config.Locking.FlushTTL
}

func (rb *ResourceBuilder) applyPersistenceConfig(resource *memcore.Resource) {
	resource.Persistence.ParsedTTL = rb.config.Persistence.ParsedTTL
}

// loadMemoryConfig loads and validates a memory configuration by ID
func (mm *Manager) loadMemoryConfig(resourceID string) (*memcore.Resource, error) {
	// Since this is greenfield, we expect properly typed configs only
	configMap, err := mm.resourceRegistry.Get("memory", resourceID)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeConfig,
			Operation:  "load",
			ResourceID: resourceID,
			Cause:      err,
		}
	}
	// Create a properly typed config from the map
	config, err := mm.createConfigFromMap(resourceID, configMap)
	if err != nil {
		return nil, err
	}
	// Build resource using the new builder pattern
	builder := &ResourceBuilder{config: config, logger: mm.log}
	return builder.Build()
}

// CacheKeyBuilder provides consistent cache key generation
type CacheKeyBuilder struct {
	namespace string
}

// NewCacheKeyBuilder creates a new cache key builder
func NewCacheKeyBuilder(namespace string) *CacheKeyBuilder {
	return &CacheKeyBuilder{namespace: namespace}
}

// ForTokenCounter generates a cache key for token counters
func (b *CacheKeyBuilder) ForTokenCounter(model string) string {
	return fmt.Sprintf("%s:token-counter:%s", b.namespace, model)
}

// ForUnifiedCounter generates a cache key for unified counters
func (b *CacheKeyBuilder) ForUnifiedCounter(provider, model, apiKeyHash string) string {
	if apiKeyHash != "" {
		return fmt.Sprintf("%s:unified-counter:%s:%s:%s", b.namespace, provider, model, apiKeyHash)
	}
	return fmt.Sprintf("%s:unified-counter:%s:%s", b.namespace, provider, model)
}

// HashAPIKey creates a hash of an API key for cache key generation
func (b *CacheKeyBuilder) HashAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	// Use first 32 characters for sufficient uniqueness
	return hex.EncodeToString(hasher.Sum(nil))[:32]
}

// resolveMemoryKey evaluates the memory key template and returns the resolved key
func (mm *Manager) resolveMemoryKey(
	_ context.Context,
	keyTemplate string,
	workflowContextData map[string]any,
) (string, string) {
	var keyToSanitize string
	// Use RenderString for plain string templates (not JSON/YAML)
	rendered, err := mm.tplEngine.RenderString(keyTemplate, workflowContextData)
	if err != nil {
		// Fall back to sanitizing the template as-is with warning
		mm.log.Warn("Failed to evaluate key template",
			"template", keyTemplate,
			"error", err)
		keyToSanitize = keyTemplate
	} else {
		keyToSanitize = rendered
	}
	// Sanitize the key (either resolved or fallback) and extract project ID
	sanitizedKey := mm.sanitizeKey(keyToSanitize)
	projectIDVal := extractProjectID(workflowContextData)
	return sanitizedKey, projectIDVal
}

// extractProjectID extracts project ID from workflow context data
// Expected key: "project.id" as a top-level string value
// Does not support nested structures like {"project": {"id": "..."}}
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
	// Use centralized cache key builder
	keyBuilder := NewCacheKeyBuilder("memory")

	// Generate cache key based on configuration
	var cacheKey string
	if providerConfig != nil {
		apiKeyHash := keyBuilder.HashAPIKey(providerConfig.APIKey)
		cacheKey = keyBuilder.ForUnifiedCounter(providerConfig.Provider, providerConfig.Model, apiKeyHash)
	} else {
		cacheKey = keyBuilder.ForTokenCounter(model)
	}

	// Try to get from cache first
	if mm.componentCache != nil {
		if cached, found := mm.componentCache.Get(cacheKey); found {
			// Extract the appropriate counter type
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

	// Create new counter if not cached
	if providerConfig != nil {
		return mm.createUnifiedCounter(model, providerConfig, cacheKey)
	}
	return mm.createTiktokenCounter(model, cacheKey)
}

// createUnifiedCounter creates a new unified token counter
func (mm *Manager) createUnifiedCounter(
	model string,
	providerConfig *memcore.TokenProviderConfig,
	cacheKey string,
) (memcore.TokenCounter, error) {
	keyResolver := tokens.NewAPIKeyResolver(mm.log)
	tokensProviderConfig := keyResolver.ResolveProviderConfig(providerConfig)
	cacheableCounter, err := NewCacheableUnifiedCounter(tokensProviderConfig, model)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeCache,
			Operation:  "create_unified_counter",
			ResourceID: model,
			Cause:      err,
		}
	}
	// Store in cache if available
	if mm.componentCache != nil {
		mm.componentCache.Set(cacheKey, cacheableCounter)
	}
	return cacheableCounter.UnifiedTokenCounter, nil
}

// createTiktokenCounter creates a new tiktoken counter
func (mm *Manager) createTiktokenCounter(model, cacheKey string) (memcore.TokenCounter, error) {
	cacheableCounter, err := NewCacheableTiktokenCounter(model)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeCache,
			Operation:  "create_tiktoken_counter",
			ResourceID: model,
			Cause:      err,
		}
	}
	// Store in cache if available
	if mm.componentCache != nil {
		mm.componentCache.Set(cacheKey, cacheableCounter)
	}
	return cacheableCounter.TiktokenCounter, nil
}

// createConfigFromMap efficiently creates a Config from a map
func (mm *Manager) createConfigFromMap(resourceID string, configMap any) (*Config, error) {
	// Handle the case where the registry already returns a typed config
	if cfg, ok := configMap.(*Config); ok {
		return cfg, nil
	}
	// For maps, we still need to use YAML conversion due to complex nested structures
	// In a greenfield project, we would enforce typed configs throughout
	rawMap, ok := configMap.(map[string]any)
	if !ok {
		return nil, &Error{
			Type:       ErrorTypeConfig,
			Operation:  "convert",
			ResourceID: resourceID,
			Cause:      fmt.Errorf("expected map[string]any, got %T", configMap),
		}
	}
	// Create config using FromMap method for consistency
	config := &Config{}
	if err := config.FromMap(rawMap); err != nil {
		return nil, &Error{
			Type:       ErrorTypeConfig,
			Operation:  "convert",
			ResourceID: resourceID,
			Cause:      err,
		}
	}
	// Validate the config
	if err := config.Validate(); err != nil {
		return nil, &Error{
			Type:       ErrorTypeConfig,
			Operation:  "validate",
			ResourceID: resourceID,
			Cause:      err,
		}
	}
	return config, nil
}
