package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/CompoZy/llm-router/engine/core"         // For core.MemoryReference, core.PathCWD
	"github.com/CompoZy/llm-router/engine/infra/cache" // For cache.LockManager, cache.RedisInterface (if needed directly)
	"github.com/CompoZy/llm-router/pkg/logger"
	"github.com/CompoZy/llm-router/pkg/tplengine" // For template evaluation
	"go.temporal.io/sdk/client"                 // For Temporal client
	// Autoload will be needed to get MemoryResource configurations
	"github.com/CompoZy/llm-router/engine/autoload"
)

// MemoryManager is responsible for creating, retrieving, and managing MemoryInstances.
// It handles memory key template evaluation and ensures that MemoryInstances are
// properly configured based on MemoryResource definitions.
type MemoryManager struct {
	resourceRegistry  *autoload.ConfigRegistry // To get MemoryResource configurations
	tplEngine         *tplengine.Engine        // For evaluating key templates
	baseLockManager   cache.LockManager        // The global lock manager (e.g., RedisLockManager)
	baseRedisClient   cache.RedisInterface     // The global Redis client (for creating MemoryStore)
	temporalClient    client.Client            // Temporal client for scheduling activities
	temporalTaskQueue string                   // Default task queue for memory activities
	log               logger.Logger

	// Pool of active MemoryInstances, keyed by their resolved instance ID.
	// This provides a way to reuse instances within the same context (e.g., workflow execution)
	// if the same resolved key is requested multiple times.
	// This is a simple in-memory pool; for cross-process/worker sharing of *instances* themselves,
	// this would need to be a shared registry or instances would be reconstructed.
	// Given MemoryInstance is relatively lightweight (holds config, clients), reconstructing is fine.
	// This pool is more for "within a single request/process lifecycle".
	// For true instance sharing across boundaries, that's what the Redis backend is for (shared state).
	// Let's simplify: this manager will primarily *construct* instances on demand.
	// Caching/pooling of Go MemoryInstance structs can be an optimization if needed.
	// For now, we'll focus on construction.
	// mu sync.RWMutex
	// activeInstances map[string]*MemoryInstance
}

// NewMemoryManagerOptions holds options for creating a MemoryManager.
type NewMemoryManagerOptions struct {
	ResourceRegistry  *autoload.ConfigRegistry
	TplEngine         *tplengine.Engine
	BaseLockManager   cache.LockManager
	BaseRedisClient   cache.RedisInterface
	TemporalClient    client.Client
	TemporalTaskQueue string
	Logger            logger.Logger
}

// NewMemoryManager creates a new MemoryManager.
func NewMemoryManager(opts NewMemoryManagerOptions) (*MemoryManager, error) {
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
		opts.Logger = logger.NewNopLogger()
	}

	return &MemoryManager{
		resourceRegistry:  opts.ResourceRegistry,
		tplEngine:         opts.TplEngine,
		baseLockManager:   opts.BaseLockManager,
		baseRedisClient:   opts.BaseRedisClient,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		log:               opts.Logger,
		// activeInstances: make(map[string]*MemoryInstance),
	}, nil
}

// GetInstance retrieves or creates a MemoryInstance based on an agent's MemoryReference
// and the current workflow execution context (used for template evaluation).
//
// Parameters:
//   - ctx: The context for the operation.
//   - agentMemoryRef: The memory.MemoryReference from the agent's config.
//   - workflowContextData: A map[string]interface{} containing data for template evaluation
//     (e.g., workflow inputs, user ID, session ID).
//
// Returns:
//   - A configured MemoryInstance.
//   - An error if the MemoryResource is not found, template evaluation fails, or instance creation fails.
func (mm *MemoryManager) GetInstance(
	ctx context.Context,
	agentMemoryRef core.MemoryReference, // Using core.MemoryReference from agent config
	workflowContextData map[string]interface{},
) (Memory, error) { // Returns the Memory interface
	mm.log.Debug("GetInstance called", "resource_id", agentMemoryRef.ID, "key_template", agentMemoryRef.Key)

	// 1. Retrieve the MemoryResource configuration
	resourceCfgAny, err := mm.resourceRegistry.Get(string(core.ConfigMemory), agentMemoryRef.ID)
	if err != nil {
		mm.log.Error("Failed to get memory resource from registry", "resource_id", agentMemoryRef.ID, "error", err)
		return nil, fmt.Errorf("memory resource '%s' not found: %w", agentMemoryRef.ID, err)
	}

	var resourceCfg *Config // memory.Config
	switch v := resourceCfgAny.(type) {
	case *Config:
		resourceCfg = v
	case map[string]interface{}: // If registry stores raw maps from autoload
		// Need to parse map into memory.Config struct
		// This requires a helper or using core.FromMapDefault carefully
		// For now, assume it's already parsed or handle parsing here.
		parsedCfg := &Config{}
		// Using a library like mapstructure or a custom parser
		// For simplicity, let's assume a helper:
		if err := core.Remarshal(v, parsedCfg); err != nil {
			mm.log.Error("Failed to parse memory resource map from registry", "resource_id", agentMemoryRef.ID, "error", err)
			return nil, fmt.Errorf("failed to parse configuration for memory resource '%s': %w", agentMemoryRef.ID, err)
		}
		resourceCfg = parsedCfg
	default:
		mm.log.Error("Invalid memory resource type in registry", "resource_id", agentMemoryRef.ID, "type", fmt.Sprintf("%T", resourceCfgAny))
		return nil, fmt.Errorf("memory resource '%s' has unexpected type in registry: %T", agentMemoryRef.ID, resourceCfgAny)
	}

	// Perform validation on the loaded resourceCfg again, as it might come from a raw map
	// The resourceCfg.Validate() should handle internal consistency.
	if err := resourceCfg.Validate(); err != nil {
		mm.log.Error("Memory resource configuration failed validation", "resource_id", resourceCfg.ID, "error", err)
		return nil, fmt.Errorf("memory resource '%s' is invalid: %w", resourceCfg.ID, err)
	}


	// 2. Evaluate the memory key template
	resolvedKey, err := mm.tplEngine.Render(ctx, agentMemoryRef.Key, workflowContextData)
	if err != nil {
		mm.log.Error("Failed to render memory key template", "template", agentMemoryRef.Key, "error", err)
		return nil, fmt.Errorf("failed to render memory key template for resource '%s': %w", agentMemoryRef.ID, err)
	}
	if resolvedKey == "" {
		return nil, fmt.Errorf("rendered memory key template for resource '%s' is empty", agentMemoryRef.ID)
	}

	// Sanitize the resolved key (as per PRD: character whitelist, length, namespacing)
	// projectID should be part of workflowContextData or manager config. Assume it's in workflowContextData.
	projectIDVal, _ := workflowContextData["project.id"].(string) // Example
	sanitizedKey := SanitizeMemoryKey(resolvedKey, projectIDVal) // SanitizeMemoryKey to be created in utils.go or here
	mm.log.Debug("Memory key resolved and sanitized", "template", agentMemoryRef.Key, "raw_resolved", resolvedKey, "sanitized", sanitizedKey)


	// 3. Construct dependencies for MemoryInstance
	//    These might be shared or created per instance group (e.g., per resource type)

	// MemoryStore: Each instance effectively gets its own namespaced view via the key.
	// The underlying Redis client is shared.
	// The keyPrefix for RedisMemoryStore could be global or resource-specific.
	// Let's use a global prefix for now, instance key provides uniqueness.
	store := NewRedisMemoryStore(mm.baseRedisClient, "compozy:") // Global prefix, sanitizedKey will make it unique

	// MemoryLockManager: Wrapper around the shared baseLockManager
	lockManagerWrapper, err := NewMemoryLockManager(mm.baseLockManager, "mlock:"+projectIDVal+":") // Namespaced lock prefix
	if err != nil {
		return nil, fmt.Errorf("failed to create memory lock manager: %w", err)
	}

	// TokenMemoryManager: Specific to this resource's config
	tokenCounter, err := NewTiktokenCounter(resourceCfg.Type.String()) // Or from a model specified in resourceCfg
	if err != nil {
		return nil, fmt.Errorf("failed to create token counter for resource '%s': %w", resourceCfg.ID, err)
	}
	tokenManager, err := NewTokenMemoryManager(resourceCfg, tokenCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to create token manager for resource '%s': %w", resourceCfg.ID, err)
	}

	// HybridFlushingStrategy: Specific to this resource's config
	var summarizer MessageSummarizer
	if resourceCfg.Flushing != nil && resourceCfg.Flushing.Type == HybridSummaryFlushing {
		// Configure RuleBasedSummarizer based on resourceCfg (e.g. KeepFirstN, KeepLastN if added to config)
		summarizer = NewRuleBasedSummarizer(tokenCounter, 1, 1) // Default keep 1 first, 1 last
	}
	flushingStrategy, err := NewHybridFlushingStrategy(resourceCfg.Flushing, summarizer, tokenManager)
	if err != nil {
		// If flushing config is nil, we might want a "NoOpFlushingStrategy" or handle nil strategy in instance
		// For now, NewHybridFlushingStrategy errors if config is nil and type is hybrid, which is fine.
		// If resourceCfg.Flushing is nil, this will error if not handled.
		// Let's assume if resourceCfg.Flushing is nil, a default non-flushing or simple FIFO strategy is implied.
		// The MemoryInstance currently expects a non-nil flushingStrategy.
		// We should ensure resourceCfg.Flushing has defaults if not specified.
		// For now, if it's nil, we create a default no-op like strategy.
		if resourceCfg.Flushing == nil {
			defaultFlushCfg := &FlushingStrategyConfig{Type: SimpleFIFOFlushing} // Default to no summarization
			flushingStrategy, _ = NewHybridFlushingStrategy(defaultFlushCfg, nil, tokenManager)
		} else {
			return nil, fmt.Errorf("failed to create flushing strategy for resource '%s': %w", resourceCfg.ID, err)
		}
	}


	// 4. Create and return the MemoryInstance
	instanceOpts := NewMemoryInstanceOptions{
		InstanceID:        sanitizedKey, // Use the sanitized, resolved key
		ResourceID:        resourceCfg.ID,
		ProjectID:         projectIDVal,
		ResourceConfig:    resourceCfg,
		Store:             store,
		LockManager:       lockManagerWrapper,
		TokenManager:      tokenManager,
		FlushingStrategy:  flushingStrategy,
		TemporalClient:    mm.temporalClient,
		TemporalTaskQueue: mm.temporalTaskQueue,
		Logger:            mm.log,
	}

	instance, err := NewMemoryInstance(instanceOpts)
	if err != nil {
		mm.log.Error("Failed to create new memory instance", "instance_id", sanitizedKey, "error", err)
		return nil, fmt.Errorf("failed to create memory instance for key '%s': %w", sanitizedKey, err)
	}

	mm.log.Info("MemoryInstance retrieved/created", "instance_id", sanitizedKey, "resource_id", resourceCfg.ID)
	return instance, nil
}


// SanitizeMemoryKey (as per PRD)
// Character whitelist: [a-zA-Z0-9-_.:], max length 512
// Automatic namespacing: compozy:{project_id}:memory:{user_defined_key}
// This should ideally be in a shared utils package or within memory package if not used elsewhere.
func SanitizeMemoryKey(userDefinedKey string, projectID string) string {
	// Basic sanitization: replace non-allowed chars with underscore
	// A more robust regex would be: regexp.MustCompile(`[^a-zA-Z0-9\-_\.:]+`)
	// For simplicity here:
	var sanitized strings.Builder
	for _, r := range userDefinedKey {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == ':' {
			sanitized.WriteRune(r)
		} else {
			sanitized.WriteRune('_')
		}
	}

	sKey := sanitized.String()
	if len(sKey) > 200 { // Max length for user part of the key (total key length also matters for Redis)
		sKey = sKey[:200]
	}

	if projectID == "" {
		projectID = "defaultproject" // Fallback project ID
	}

	// Final namespaced key
	// The "compozy:" prefix for the store is already handled by RedisMemoryStore.
	// So this sanitizedKey is what goes *after* that store-level prefix.
	// Let's adjust: MemoryManager creates the *full instance ID*.
	// RedisMemoryStore then uses this full ID without adding its own prefix.
	// OR RedisMemoryStore prepends "compozy:", and this adds "{project_id}:memory:{sKey}"
	// Let's go with: Manager makes "{project_id}:memory:{sKey}" part. Store prefixes "compozy:".
	// This means MemoryInstance.id will be "{project_id}:memory:{sKey}".
	// No, the PRD says "compozy:{project_id}:memory:{user_defined_key}" is the *final* key.
	// So this function should produce that.

	finalKey := fmt.Sprintf("compozy:%s:memory:%s", projectID, sKey)
	if len(finalKey) > 512 { // Overall max length
		// This is tricky, need to truncate carefully.
		// For now, assume projectID and prefixes are short enough.
		// A better way is to hash if too long.
		// For now, simple truncation of the user part if final key is too long.
		maxUserKeyLen := 512 - len(fmt.Sprintf("compozy:%s:memory:", projectID))
		if maxUserKeyLen < 10 { maxUserKeyLen = 10} // Ensure some space for user key
		if len(sKey) > maxUserKeyLen {
			sKey = sKey[:maxUserKeyLen]
		}
		finalKey = fmt.Sprintf("compozy:%s:memory:%s", projectID, sKey)
	}
	return finalKey
}

// Helper for core.Remarshal if not available (map to struct)
// func remarshal(source map[string]interface{}, dest interface{}) error { ... }
// This is often done with json.Marshal then json.Unmarshal, or mapstructure.
// Assuming core.Remarshal exists and uses mapstructure or similar.
// If not, it needs to be implemented. For now, it's in core.
// For MemoryType.String()
func (mt MemoryType) String() string {
	return string(mt)
}
