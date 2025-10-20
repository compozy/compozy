package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/memory/tokens"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

// validKeyPattern is a compiled regex for validating memory keys
// Allow alphanumeric characters, hyphens, underscores, colons, dots, @ symbols, and asterisks
// Length limit: 1-256 characters
var validKeyPattern = regexp.MustCompile(`^[\w:\-@\.\*]{1,256}$`)

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
}

// Build constructs a memcore.Resource with all necessary mappings
func (rb *ResourceBuilder) Build(ctx context.Context) (*memcore.Resource, error) {
	log := logger.FromContext(ctx)
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
	if err := rb.applyPrivacyPolicy(ctx, resource); err != nil {
		return nil, err
	}
	// Apply locking configuration
	rb.applyLockingConfig(resource)
	// Apply persistence configuration
	rb.applyPersistenceConfig(resource)
	// Log the conversion
	log.Debug("Config to resource conversion",
		"config_ttl", rb.config.Persistence.TTL,
		"parsed_ttl", rb.config.Persistence.ParsedTTL,
		"resource_id", resource.ID)
	return resource, nil
}

func (rb *ResourceBuilder) applyPrivacyPolicy(ctx context.Context, resource *memcore.Resource) error {
	log := logger.FromContext(ctx)
	if rb.config.PrivacyPolicy == nil {
		return nil
	}
	resource.PrivacyPolicy = &memcore.PrivacyPolicyConfig{
		NonPersistableMessageTypes: rb.config.PrivacyPolicy.NonPersistableMessageTypes,
		DefaultRedactionString:     rb.config.PrivacyPolicy.DefaultRedactionString,
	}
	if len(rb.config.PrivacyPolicy.RedactPatterns) > 0 {
		// Validate and use regex patterns directly
		if err := privacy.ValidateRedactionPatterns(rb.config.PrivacyPolicy.RedactPatterns); err != nil {
			return &Error{
				Type:       ErrorTypeConfig,
				Operation:  "privacy_policy_validation",
				ResourceID: rb.config.ID,
				Cause:      err,
			}
		}
		resource.PrivacyPolicy.RedactPatterns = rb.config.PrivacyPolicy.RedactPatterns
		log.Debug("Using redaction patterns",
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
func (mm *Manager) loadMemoryConfig(ctx context.Context, resourceID string) (*memcore.Resource, error) {
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
	config, err := mm.createConfigFromMap(ctx, resourceID, configMap)
	if err != nil {
		return nil, err
	}
	// Build resource using the new builder pattern
	builder := &ResourceBuilder{config: config}
	return builder.Build(ctx)
}

// resolveMemoryKey evaluates the memory key template and returns the resolved key
func (mm *Manager) resolveMemoryKey(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (string, error) {
	log := logger.FromContext(ctx)
	// Extract project ID early
	projectID := mm.projectContextResolver.ResolveProjectID(ctx, workflowContextData)
	// Get the key to validate
	keyToValidate := mm.getKeyToValidate(ctx, agentMemoryRef, workflowContextData)
	// Validate the key
	validatedKey, err := mm.validateKey(keyToValidate)
	if err != nil {
		// Extract workflow execution ID for correlation
		workflowExecID := ExtractWorkflowExecID(workflowContextData)
		return "", fmt.Errorf("memory key validation failed for '%s' (project: %s, workflow_exec_id: %s): %w",
			keyToValidate, projectID, workflowExecID, err)
	}
	log.Debug("Memory key resolution complete",
		"original_template", agentMemoryRef.Key,
		"resolved_key", agentMemoryRef.ResolvedKey,
		"validated_key", validatedKey,
		"project_id", projectID)
	return validatedKey, nil
}

// getProjectID extracts project ID from workflow context data using the centralized resolver
func (mm *Manager) getProjectID(ctx context.Context, workflowContextData map[string]any) string {
	return mm.projectContextResolver.ResolveProjectID(ctx, workflowContextData)
}

// getKeyToValidate determines which key to use based on the reference type
func (mm *Manager) getKeyToValidate(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) string {
	// Use pre-resolved key if available (e.g., from REST API)
	if agentMemoryRef.ResolvedKey != "" {
		log := logger.FromContext(ctx)
		log.Debug("Using pre-resolved key", "key", agentMemoryRef.ResolvedKey)
		return agentMemoryRef.ResolvedKey
	}
	// Determine template to use: agent-specified key, or fallback to
	// memory.Config.default_key_template if agent key is empty.
	keyTemplate := agentMemoryRef.Key
	if keyTemplate == "" {
		log := logger.FromContext(ctx)
		// Attempt to load memory config to retrieve default key template
		if mm.resourceRegistry != nil {
			if configMap, err := mm.resourceRegistry.Get("memory", agentMemoryRef.ID); err == nil {
				if cfg, err2 := mm.createConfigFromMap(ctx, agentMemoryRef.ID, configMap); err2 == nil && cfg != nil {
					if cfg.DefaultKeyTemplate != "" {
						keyTemplate = cfg.DefaultKeyTemplate
						log.Debug("Using default_key_template from memory config",
							"memory_id", agentMemoryRef.ID,
							"template", keyTemplate,
						)
					}
				}
			}
		}
	}
	// Resolve from template (may still be empty and will be validated later)
	return mm.resolveKeyFromTemplate(ctx, keyTemplate, agentMemoryRef.ID, workflowContextData)
}

// resolveKeyFromTemplate handles template resolution for memory keys
func (mm *Manager) resolveKeyFromTemplate(
	ctx context.Context,
	keyTemplate string,
	_ string,
	workflowContextData map[string]any,
) string {
	log := logger.FromContext(ctx)
	if !tplengine.HasTemplate(keyTemplate) {
		log.Debug("Using literal key (no template syntax detected)", "key", keyTemplate)
		return keyTemplate
	}
	// Attempt template resolution
	log.Debug("Attempting template resolution",
		"template", keyTemplate,
		"has_template_engine", mm.tplEngine != nil)
	if mm.tplEngine == nil {
		log.Error("Template engine not available for key resolution", "template", keyTemplate)
		return keyTemplate // Return original for validation error
	}
	rendered, err := mm.tplEngine.RenderString(keyTemplate, workflowContextData)
	if err != nil {
		log.Error("Failed to evaluate key template",
			"template", keyTemplate,
			"error", err)
		// Return template as-is to trigger validation error
		// This ensures template resolution errors are properly propagated
		return keyTemplate
	}
	log.Debug("Template resolved successfully",
		"template", keyTemplate,
		"rendered", rendered)
	return rendered
}

// ProjectContextResolver provides centralized project ID resolution
type ProjectContextResolver struct {
	fallbackProjectID string
}

// NewProjectContextResolver creates a resolver with a fallback project ID
func NewProjectContextResolver(fallbackProjectID string) *ProjectContextResolver {
	return &ProjectContextResolver{
		fallbackProjectID: fallbackProjectID,
	}
}

// ResolveProjectID extracts project ID from workflow context with fallback
func (r *ProjectContextResolver) ResolveProjectID(ctx context.Context, workflowContextData map[string]any) string {
	log := logger.FromContext(ctx)
	// Try nested format first (standard workflow format)
	if project, ok := workflowContextData["project"]; ok {
		if projectMap, ok := project.(map[string]any); ok {
			if id, ok := projectMap["id"]; ok {
				if idStr, ok := id.(string); ok && idStr != "" {
					log.Info("Project ID resolved from nested format", "project_id", idStr)
					return idStr
				}
			}
		}
	}
	// Try flat format (legacy support)
	if projectID, ok := workflowContextData["project.id"]; ok {
		if projectIDStr, ok := projectID.(string); ok && projectIDStr != "" {
			log.Info("Project ID resolved from flat format", "project_id", projectIDStr)
			return projectIDStr
		}
	}
	// Use fallback project ID (from appState.ProjectConfig.Name)
	log.Info("Using fallback project ID", "fallback_project_id", r.fallbackProjectID)
	return r.fallbackProjectID
}

// ExtractWorkflowExecID extracts the workflow execution ID from workflow context data
func ExtractWorkflowExecID(contextData map[string]any) string {
	if contextData == nil {
		return "unknown"
	}
	// Check for workflow.exec_id
	if workflow, ok := contextData["workflow"].(map[string]any); ok {
		if execID, ok := workflow["exec_id"].(string); ok && execID != "" {
			return execID
		}
	}
	return "unknown"
}

// validateKey validates that a memory key is safe for Redis storage
// and returns the key unchanged if valid, or an error if invalid
func (mm *Manager) validateKey(key string) (string, error) {
	if !validKeyPattern.MatchString(key) {
		return "", fmt.Errorf(
			"invalid memory key '%s': must contain only alphanumeric characters, "+
				"hyphens, underscores, colons, dots, @ symbols, and asterisks, and be 1-256 characters long",
			key,
		)
	}
	// Additional validation: prevent keys that might conflict with Redis internals
	if strings.HasPrefix(key, "__") || strings.HasSuffix(key, "__") {
		return "", fmt.Errorf("invalid memory key '%s': keys cannot start or end with '__'", key)
	}
	return key, nil
}

// registerPrivacyPolicy registers the privacy policy if one is configured
func (mm *Manager) registerPrivacyPolicy(ctx context.Context, resourceCfg *memcore.Resource) error {
	log := logger.FromContext(ctx)
	if resourceCfg.PrivacyPolicy != nil {
		if err := mm.privacyManager.RegisterPolicy(ctx, resourceCfg.ID, resourceCfg.PrivacyPolicy); err != nil {
			log.Error("Failed to register privacy policy", "resource_id", resourceCfg.ID, "error", err)
			return fmt.Errorf("failed to register privacy policy for resource '%s': %w", resourceCfg.ID, err)
		}
	}
	return nil
}

// getOrCreateTokenCounter retrieves or creates a token counter for the given model
func (mm *Manager) getOrCreateTokenCounter(ctx context.Context, model string) (memcore.TokenCounter, error) {
	return mm.getOrCreateTokenCounterWithConfig(ctx, model, nil)
}

// getOrCreateTokenCounterWithConfig retrieves or creates a token counter for
// the given model and optional provider config
func (mm *Manager) getOrCreateTokenCounterWithConfig(
	ctx context.Context,
	model string,
	providerConfig *memcore.TokenProviderConfig,
) (memcore.TokenCounter, error) {
	// Create new counter directly without caching
	if providerConfig != nil {
		return mm.createUnifiedCounter(ctx, model, providerConfig)
	}
	return mm.createTiktokenCounter(model)
}

// createUnifiedCounter creates a new unified token counter
func (mm *Manager) createUnifiedCounter(
	ctx context.Context,
	model string,
	providerConfig *memcore.TokenProviderConfig,
) (memcore.TokenCounter, error) {
	keyResolver := tokens.NewAPIKeyResolver()
	tokensProviderConfig := keyResolver.ResolveProviderConfig(ctx, providerConfig)
	// Create fallback counter
	fallback, err := tokens.NewTiktokenCounter(model)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeCache,
			Operation:  "create_fallback_counter",
			ResourceID: model,
			Cause:      err,
		}
	}
	// Create unified counter
	counter, err := tokens.NewUnifiedTokenCounter(tokensProviderConfig, fallback)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeCache,
			Operation:  "create_unified_counter",
			ResourceID: model,
			Cause:      err,
		}
	}
	return counter, nil
}

// createTiktokenCounter creates a new tiktoken counter
func (mm *Manager) createTiktokenCounter(model string) (memcore.TokenCounter, error) {
	counter, err := tokens.NewTiktokenCounter(model)
	if err != nil {
		return nil, &Error{
			Type:       ErrorTypeCache,
			Operation:  "create_tiktoken_counter",
			ResourceID: model,
			Cause:      err,
		}
	}
	return counter, nil
}

// createConfigFromMap efficiently creates a Config from a map
func (mm *Manager) createConfigFromMap(ctx context.Context, resourceID string, configMap any) (*Config, error) {
	// Handle the case where the registry already returns a typed config
	if cfg, ok := configMap.(*Config); ok {
		cloned := cloneConfigForValidation(cfg)
		if err := cloned.Validate(ctx); err != nil {
			return nil, &Error{
				Type:       ErrorTypeConfig,
				Operation:  "validate",
				ResourceID: resourceID,
				Cause:      err,
			}
		}
		return cloned, nil
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
	if err := config.Validate(ctx); err != nil {
		return nil, &Error{
			Type:       ErrorTypeConfig,
			Operation:  "validate",
			ResourceID: resourceID,
			Cause:      err,
		}
	}
	return config, nil
}

func cloneConfigForValidation(cfg *Config) *Config {
	cloned := &Config{
		Resource:           cfg.Resource,
		ID:                 cfg.ID,
		Description:        cfg.Description,
		Version:            cfg.Version,
		Type:               cfg.Type,
		MaxTokens:          cfg.MaxTokens,
		MaxMessages:        cfg.MaxMessages,
		MaxContextRatio:    cfg.MaxContextRatio,
		Persistence:        cfg.Persistence,
		DefaultKeyTemplate: cfg.DefaultKeyTemplate,
		filePath:           cfg.filePath,
		ttlManager:         cfg.ttlManager,
	}
	cloned.TokenAllocation = cloneTokenAllocation(cfg.TokenAllocation)
	cloned.Flushing = cloneFlushingConfig(cfg.Flushing)
	cloned.PrivacyPolicy = clonePrivacyPolicy(cfg.PrivacyPolicy)
	cloned.Locking = cloneLockingConfig(cfg.Locking)
	cloned.TokenProvider = cloneTokenProvider(cfg.TokenProvider)
	cloned.CWD = cloneCWDConfig(cfg.CWD)
	return cloned
}

func cloneTokenAllocation(allocation *memcore.TokenAllocation) *memcore.TokenAllocation {
	if allocation == nil {
		return nil
	}
	copyAllocation := *allocation
	if len(allocation.UserDefined) > 0 {
		copyAllocation.UserDefined = core.CloneMap(allocation.UserDefined)
	}
	return &copyAllocation
}

func cloneFlushingConfig(flushing *memcore.FlushingStrategyConfig) *memcore.FlushingStrategyConfig {
	if flushing == nil {
		return nil
	}
	copyFlushing := *flushing
	return &copyFlushing
}

func clonePrivacyPolicy(policy *memcore.PrivacyPolicyConfig) *memcore.PrivacyPolicyConfig {
	if policy == nil {
		return nil
	}
	copyPolicy := *policy
	if len(policy.RedactPatterns) > 0 {
		copyPolicy.RedactPatterns = append([]string(nil), policy.RedactPatterns...)
	}
	if len(policy.NonPersistableMessageTypes) > 0 {
		copyPolicy.NonPersistableMessageTypes = append(
			[]string(nil),
			policy.NonPersistableMessageTypes...)
	}
	return &copyPolicy
}

func cloneLockingConfig(locking *memcore.LockConfig) *memcore.LockConfig {
	if locking == nil {
		return nil
	}
	copyLocking := *locking
	return &copyLocking
}

func cloneTokenProvider(provider *memcore.TokenProviderConfig) *memcore.TokenProviderConfig {
	if provider == nil {
		return nil
	}
	copyProvider := *provider
	if len(provider.Settings) > 0 {
		copyProvider.Settings = core.CloneMap(provider.Settings)
	}
	return &copyProvider
}

func cloneCWDConfig(cwd *core.PathCWD) *core.PathCWD {
	if cwd == nil {
		return nil
	}
	copyCWD := *cwd
	return &copyCWD
}
