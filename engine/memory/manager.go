package memory

import (
	"context"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"go.temporal.io/sdk/client"
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
	privacyManager    privacy.ManagerInterface  // Privacy controls and data protection
	log               logger.Logger             // Logger following the project standard with log.FromContext(ctx)
	componentCache    *componentCache           // Component cache for performance optimization
}

// ManagerOptions holds options for creating a Manager.
type ManagerOptions struct {
	ResourceRegistry        *autoload.ConfigRegistry
	TplEngine               *tplengine.TemplateEngine
	BaseLockManager         cache.LockManager
	BaseRedisClient         cache.RedisInterface
	TemporalClient          client.Client
	TemporalTaskQueue       string
	PrivacyManager          privacy.ManagerInterface  // Optional: if nil, a new one will be created
	PrivacyResilienceConfig *privacy.ResilienceConfig // Optional: if provided, creates resilient privacy manager
	Logger                  logger.Logger
	ComponentCacheConfig    *ComponentCacheConfig // Optional: if nil, default config will be used
	DisableComponentCache   bool                  // Optional: disable component caching entirely
}

// NewManager creates a new Manager.
func NewManager(opts *ManagerOptions) (*Manager, error) {
	if err := validateManagerOptions(opts); err != nil {
		return nil, err
	}
	setDefaultManagerOptions(opts)
	privacyManager := getOrCreatePrivacyManager(opts.PrivacyManager, opts.PrivacyResilienceConfig, opts.Logger)
	cache := createComponentCache(opts)
	return &Manager{
		resourceRegistry:  opts.ResourceRegistry,
		tplEngine:         opts.TplEngine,
		baseLockManager:   opts.BaseLockManager,
		baseRedisClient:   opts.BaseRedisClient,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    privacyManager,
		log:               opts.Logger,
		componentCache:    cache,
	}, nil
}

// GetInstance retrieves or creates a MemoryInstance based on an agent's MemoryReference
// and the current workflow execution context (used for template evaluation).
func (mm *Manager) GetInstance(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (memcore.Memory, error) {
	mm.log.Debug("GetInstance called", "resource_id", agentMemoryRef.ID, "key_template", agentMemoryRef.Key)
	resourceCfg, err := mm.loadMemoryConfig(agentMemoryRef.ID)
	if err != nil {
		return nil, err
	}
	sanitizedKey, projectIDVal := mm.resolveMemoryKey(ctx, agentMemoryRef.Key, workflowContextData)
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
