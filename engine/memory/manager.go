package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"        // For core.MemoryReference, core.PathCWD
	"github.com/compozy/compozy/engine/infra/cache" // For cache.LockManager, cache.RedisInterface (if needed directly)
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine" // For template evaluation
	"go.temporal.io/sdk/client"                // For Temporal client

	// Autoload will be needed to get Resource configurations
	"github.com/compozy/compozy/engine/autoload"
)

const (
	// DefaultTokenCounterModel is the default model used for token counting
	DefaultTokenCounterModel = "gpt-4"
)

// Manager is responsible for creating, retrieving, and managing Instances.
// It handles memory key template evaluation and ensures that Instances are
// properly configured based on Resource definitions.
type Manager struct {
	resourceRegistry  *autoload.ConfigRegistry  // To get MemoryResource configurations
	tplEngine         *tplengine.TemplateEngine // For evaluating key templates
	baseLockManager   cache.LockManager         // The global lock manager (e.g., RedisLockManager)
	baseRedisClient   cache.RedisInterface      // The global Redis client (for creating Store)
	temporalClient    client.Client             // Temporal client for scheduling activities
	temporalTaskQueue string                    // Default task queue for memory activities
	privacyManager    *PrivacyManager           // Privacy controls and data protection
	log               logger.Logger

	// TODO: Component cache for performance optimization
	// This feature is not yet implemented but would cache stateless components
	// that depend only on resource configuration
}

// ManagerOptions holds options for creating a Manager.
type ManagerOptions struct {
	ResourceRegistry  *autoload.ConfigRegistry
	TplEngine         *tplengine.TemplateEngine
	BaseLockManager   cache.LockManager
	BaseRedisClient   cache.RedisInterface
	TemporalClient    client.Client
	TemporalTaskQueue string
	PrivacyManager    *PrivacyManager // Optional: if nil, a new one will be created
	Logger            logger.Logger
}

// NewManager creates a new Manager.
func NewManager(opts *ManagerOptions) (*Manager, error) {
	if opts.ResourceRegistry == nil {
		return nil, fmt.Errorf("resource registry cannot be nil")
	}
	if opts.TplEngine == nil {
		return nil, fmt.Errorf("template engine cannot be nil")
	}
	if opts.BaseLockManager == nil {
		return nil, fmt.Errorf("base lock manager cannot be nil")
	}
	if opts.BaseRedisClient == nil {
		return nil, fmt.Errorf("base redis client cannot be nil")
	}
	if opts.TemporalClient == nil {
		return nil, fmt.Errorf("temporal client cannot be nil")
	}
	if opts.TemporalTaskQueue == "" {
		opts.TemporalTaskQueue = "memory-operations" // Default
	}
	if opts.Logger == nil {
		opts.Logger = logger.NewForTests()
	}

	// Create privacy manager if not provided
	privacyManager := opts.PrivacyManager
	if privacyManager == nil {
		privacyManager = NewPrivacyManager()
	}
	return &Manager{
		resourceRegistry:  opts.ResourceRegistry,
		tplEngine:         opts.TplEngine,
		baseLockManager:   opts.BaseLockManager,
		baseRedisClient:   opts.BaseRedisClient,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    privacyManager,
		log:               opts.Logger,
	}, nil
}

// GetInstance retrieves or creates a MemoryInstance based on an agent's MemoryReference
// and the current workflow execution context (used for template evaluation).
//
// Parameters:
//   - ctx: The context for the operation.
//   - agentMemoryRef: The core.MemoryReference from the agent's config.
//   - workflowContextData: A map[string]interface{} containing data for template evaluation
//     (e.g., workflow inputs, user ID, session ID).
//
// Returns:
//   - A configured MemoryInstance.
//   - An error if the MemoryResource is not found, template evaluation fails, or instance creation fails.
func (mm *Manager) GetInstance(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (Memory, error) {
	mm.log.Debug("GetInstance called", "resource_id", agentMemoryRef.ID, "key_template", agentMemoryRef.Key)
	resourceCfg, err := mm.loadMemoryConfig(agentMemoryRef.ID)
	if err != nil {
		return nil, err
	}
	sanitizedKey, projectIDVal, err := mm.resolveMemoryKey(ctx, agentMemoryRef.Key, workflowContextData)
	if err != nil {
		return nil, err
	}
	components, err := mm.buildMemoryComponents(resourceCfg, projectIDVal)
	if err != nil {
		return nil, err
	}
	err = mm.registerPrivacyPolicy(resourceCfg)
	if err != nil {
		return nil, err
	}
	instance, err := mm.createMemoryInstance(sanitizedKey, projectIDVal, resourceCfg, components)
	if err != nil {
		return nil, err
	}
	mm.log.Info("MemoryInstance retrieved/created", "instance_id", sanitizedKey, "resource_id", resourceCfg.ID)
	return instance, nil
}

// memoryComponents holds all the dependencies needed for a memory instance
type memoryComponents struct {
	store            Store
	lockManager      *LockManager
	tokenManager     *TokenMemoryManager
	flushingStrategy *HybridFlushingStrategy
}

// buildMemoryComponents creates all the necessary components for a memory instance
func (mm *Manager) buildMemoryComponents(resourceCfg *Config, projectIDVal string) (*memoryComponents, error) {
	store := NewRedisMemoryStore(mm.baseRedisClient, "")
	lockManager, err := mm.createLockManager(projectIDVal)
	if err != nil {
		return nil, err
	}
	tokenManager, err := mm.createTokenManager(resourceCfg)
	if err != nil {
		return nil, err
	}
	flushingStrategy, err := mm.createFlushingStrategy(resourceCfg, tokenManager)
	if err != nil {
		return nil, err
	}
	return &memoryComponents{
		store:            store,
		lockManager:      lockManager,
		tokenManager:     tokenManager,
		flushingStrategy: flushingStrategy,
	}, nil
}

// createLockManager creates a memory lock manager with project namespacing
func (mm *Manager) createLockManager(projectIDVal string) (*LockManager, error) {
	lockManagerWrapper, err := NewLockManager(
		mm.baseLockManager,
		"mlock:"+projectIDVal+":",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory lock manager: %w", err)
	}
	return lockManagerWrapper, nil
}

// createTokenManager creates a token manager for the given resource configuration
func (mm *Manager) createTokenManager(resourceCfg *Config) (*TokenMemoryManager, error) {
	model := DefaultTokenCounterModel
	tokenCounter, err := NewTiktokenCounter(model)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create token counter for resource '%s' with model '%s': %w",
			resourceCfg.ID,
			model,
			err,
		)
	}
	memResource := mm.convertConfigToMemoryResource(resourceCfg)
	tokenManager, err := NewTokenMemoryManager(memResource, tokenCounter, mm.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create token manager for resource '%s': %w", resourceCfg.ID, err)
	}
	return tokenManager, nil
}

// convertConfigToMemoryResource converts a Config to a MemoryResource
func (mm *Manager) convertConfigToMemoryResource(resourceCfg *Config) *Resource {
	return &Resource{
		ID:               resourceCfg.ID,
		Description:      resourceCfg.Description,
		Type:             resourceCfg.Type,
		MaxTokens:        resourceCfg.MaxTokens,
		MaxMessages:      resourceCfg.MaxMessages,
		MaxContextRatio:  resourceCfg.MaxContextRatio,
		TokenAllocation:  resourceCfg.TokenAllocation,
		FlushingStrategy: resourceCfg.Flushing,
		Persistence:      resourceCfg.Persistence,
		PrivacyPolicy:    resourceCfg.PrivacyPolicy,
		Version:          resourceCfg.Version,
	}
}

// createFlushingStrategy creates a flushing strategy for the given resource configuration
func (mm *Manager) createFlushingStrategy(
	resourceCfg *Config,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	var summarizer MessageSummarizer
	if resourceCfg.Flushing != nil && resourceCfg.Flushing.Type == HybridSummaryFlushing {
		tokenCounter, err := NewTiktokenCounter(DefaultTokenCounterModel)
		if err != nil {
			return nil, fmt.Errorf("failed to create token counter for summarizer: %w", err)
		}
		summarizer = NewRuleBasedSummarizer(tokenCounter, 1, 1)
	}
	flushingStrategy, err := NewHybridFlushingStrategy(resourceCfg.Flushing, summarizer, tokenManager)
	if err != nil {
		if resourceCfg.Flushing == nil {
			return mm.createDefaultFlushingStrategy(resourceCfg, tokenManager)
		}
		return nil, fmt.Errorf("failed to create flushing strategy for resource '%s': %w", resourceCfg.ID, err)
	}
	return flushingStrategy, nil
}

// createDefaultFlushingStrategy creates a default FIFO flushing strategy when none is configured
func (mm *Manager) createDefaultFlushingStrategy(
	resourceCfg *Config,
	tokenManager *TokenMemoryManager,
) (*HybridFlushingStrategy, error) {
	defaultFlushCfg := &FlushingStrategyConfig{Type: SimpleFIFOFlushing}
	flushingStrategy, err := NewHybridFlushingStrategy(defaultFlushCfg, nil, tokenManager)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create default flushing strategy for resource '%s': %w",
			resourceCfg.ID,
			err,
		)
	}
	return flushingStrategy, nil
}

// registerPrivacyPolicy registers the privacy policy if one is configured
func (mm *Manager) registerPrivacyPolicy(resourceCfg *Config) error {
	if resourceCfg.PrivacyPolicy != nil {
		if err := mm.privacyManager.RegisterPolicy(resourceCfg.ID, resourceCfg.PrivacyPolicy); err != nil {
			mm.log.Error("Failed to register privacy policy", "resource_id", resourceCfg.ID, "error", err)
			return fmt.Errorf("failed to register privacy policy for resource '%s': %w", resourceCfg.ID, err)
		}
	}
	return nil
}

// createMemoryInstance creates the final memory instance with all components
func (mm *Manager) createMemoryInstance(
	sanitizedKey, projectIDVal string,
	resourceCfg *Config,
	components *memoryComponents,
) (Memory, error) {
	instanceOpts := NewInstanceOptions{
		InstanceID:        sanitizedKey,
		ResourceID:        resourceCfg.ID,
		ProjectID:         projectIDVal,
		ResourceConfig:    resourceCfg,
		Store:             components.store,
		LockManager:       components.lockManager,
		TokenManager:      components.tokenManager,
		FlushingStrategy:  components.flushingStrategy,
		TemporalClient:    mm.temporalClient,
		TemporalTaskQueue: mm.temporalTaskQueue,
		PrivacyManager:    mm.privacyManager,
		Logger:            mm.log,
	}
	instance, err := NewInstance(&instanceOpts)
	if err != nil {
		mm.log.Error("Failed to create new memory instance", "instance_id", sanitizedKey, "error", err)
		return nil, fmt.Errorf("failed to create memory instance for key '%s': %w", sanitizedKey, err)
	}
	return instance, nil
}

func (mm *Manager) loadMemoryConfig(resourceID string) (*Config, error) {
	resourceCfgAny, err := mm.resourceRegistry.Get(string(core.ConfigMemory), resourceID)
	if err != nil {
		mm.log.Error("Failed to get memory resource from registry", "resource_id", resourceID, "error", err)
		return nil, fmt.Errorf("memory resource '%s' not found: %w", resourceID, err)
	}
	switch v := resourceCfgAny.(type) {
	case *Config:
		if err := v.Validate(); err != nil {
			mm.log.Error("Memory resource configuration failed validation", "resource_id", v.ID, "error", err)
			return nil, fmt.Errorf("memory resource '%s' is invalid: %w", v.ID, err)
		}
		return v, nil
	case map[string]any:
		parsedCfg := &Config{}
		if err := remarshal(v, parsedCfg); err != nil {
			mm.log.Error("Failed to parse memory resource map", "resource_id", resourceID, "error", err)
			return nil, fmt.Errorf("failed to parse configuration for memory resource '%s': %w", resourceID, err)
		}
		if err := parsedCfg.Validate(); err != nil {
			mm.log.Error("Memory resource configuration failed validation", "resource_id", parsedCfg.ID, "error", err)
			return nil, fmt.Errorf("memory resource '%s' is invalid: %w", parsedCfg.ID, err)
		}
		return parsedCfg, nil
	default:
		mm.log.Error("Invalid memory resource type", "resource_id", resourceID, "type", fmt.Sprintf("%T", v))
		return nil, fmt.Errorf("memory resource '%s' has unexpected type: %T", resourceID, v)
	}
}

func (mm *Manager) resolveMemoryKey(
	ctx context.Context,
	keyTemplate string,
	workflowContextData map[string]any,
) (sanitizedKey string, projectID string, err error) {
	resolvedKey, err := mm.tplEngine.RenderString(keyTemplate, workflowContextData)
	if err != nil {
		mm.log.Error("Failed to render memory key template", "template", keyTemplate, "error", err)
		return "", "", fmt.Errorf("failed to render memory key template: %w", err)
	}
	if resolvedKey == "" {
		return "", "", fmt.Errorf("rendered memory key template is empty")
	}
	projectIDVal, ok := workflowContextData["project.id"].(string)
	if !ok || projectIDVal == "" {
		return "", "", fmt.Errorf("project.id not found or is empty in workflow context")
	}
	sanitizedKey, err = SanitizeMemoryKey(resolvedKey, projectIDVal)
	if err != nil {
		mm.log.Error("Failed to sanitize memory key", "error", err, "key", resolvedKey, "project_id", projectIDVal)
		return "", "", err
	}
	mm.log.Debug("Memory key resolved and sanitized",
		"template", keyTemplate,
		"raw_resolved", resolvedKey,
		"sanitized", sanitizedKey)
	RecordConfigResolution(ctx, keyTemplate, projectIDVal)
	return sanitizedKey, projectIDVal, nil
}

// SanitizeMemoryKey (as per PRD)
// Character whitelist: [a-zA-Z0-9-_.:], max length 512
// Automatic namespacing: compozy:{project_id}:memory:{user_defined_key}
// This should ideally be in a shared utils package or within memory package if not used elsewhere.
func SanitizeMemoryKey(userDefinedKey string, projectID string) (string, error) {
	sKey := sanitizeUserKey(userDefinedKey)
	if projectID == "" {
		// Defensive programming: project ID should never be empty at this point
		// as it's validated upstream in resolveMemoryKey
		return "", fmt.Errorf(
			"BUG: SanitizeMemoryKey called with empty projectID - this should have been caught earlier",
		)
	}
	return buildFinalKey(projectID, sKey), nil
}

func sanitizeUserKey(userDefinedKey string) string {
	var sanitized strings.Builder
	for _, r := range userDefinedKey {
		if isAllowedChar(r) {
			sanitized.WriteRune(r)
		} else {
			sanitized.WriteRune('_')
		}
	}
	sKey := sanitized.String()
	if len(sKey) > 200 {
		sKey = sKey[:200]
	}
	return sKey
}

func isAllowedChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.' || r == ':'
}

func buildFinalKey(projectID, sKey string) string {
	prefix := fmt.Sprintf("compozy:%s:memory:", projectID)
	finalKey := prefix + sKey
	if len(finalKey) <= 512 {
		return finalKey
	}
	// If the key is too long, hash the user-defined part to prevent collisions
	hash := sha256.Sum256([]byte(sKey))
	hashedKey := hex.EncodeToString(hash[:]) // Use full hash for collision resistance
	finalHashedKey := prefix + hashedKey
	if len(finalHashedKey) > 512 {
		// This should not happen with a fixed-size hash, but as a safeguard
		return finalHashedKey[:512]
	}
	return finalHashedKey
}

// TODO: Implement component caching for performance optimization
// This would cache expensive components like TokenCounter that are stateless
// and can be reused across multiple GetInstance calls for the same resource

// remarshal converts a map to a struct using JSON marshaling
func remarshal(source any, dest any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("failed to marshal source: %w", err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal to destination: %w", err)
	}
	return nil
}
