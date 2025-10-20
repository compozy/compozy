package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/engine/memory/tokens"
	"github.com/compozy/compozy/pkg/config"
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
	resourceRegistry       *autoload.ConfigRegistry  // To get MemoryResource configurations
	tplEngine              *tplengine.TemplateEngine // For evaluating key templates
	baseLockManager        cache.LockManager         // The global lock manager (e.g., RedisLockManager)
	baseRedisClient        cache.RedisInterface      // The global Redis client (for creating Store)
	temporalClient         client.Client             // Temporal client for scheduling activities
	temporalTaskQueue      string                    // Default task queue for memory activities
	privacyManager         privacy.ManagerInterface  // Privacy controls and data protection
	projectContextResolver *ProjectContextResolver   // For consistent project ID resolution
	appConfig              *config.Config            // Configuration manager for accessing app config
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
	FallbackProjectID       string                    // Project ID to use when not found in context
	AppConfig               *config.Config            // Optional: for accessing application configuration
}

// NewManager creates a new Manager.
func NewManager(opts *ManagerOptions) (*Manager, error) {
	if err := validateManagerOptions(opts); err != nil {
		return nil, err
	}
	setDefaultManagerOptions(opts)
	privacyManager := getOrCreatePrivacyManager(opts.PrivacyManager, opts.PrivacyResilienceConfig)
	projectContextResolver := NewProjectContextResolver(opts.FallbackProjectID)
	return &Manager{
		resourceRegistry:       opts.ResourceRegistry,
		tplEngine:              opts.TplEngine,
		baseLockManager:        opts.BaseLockManager,
		baseRedisClient:        opts.BaseRedisClient,
		temporalClient:         opts.TemporalClient,
		temporalTaskQueue:      opts.TemporalTaskQueue,
		privacyManager:         privacyManager,
		projectContextResolver: projectContextResolver,
		appConfig:              opts.AppConfig,
	}, nil
}

// GetInstance retrieves or creates a MemoryInstance based on an agent's MemoryReference
// and the current workflow execution context (used for template evaluation).
func (mm *Manager) GetInstance(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (memcore.Memory, error) {
	log := logger.FromContext(ctx)
	// Retry logic for transient key resolution failures using go-retry
	var instance memcore.Memory
	retryConfig := retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond))
	err := retry.Do(ctx, retryConfig, func(ctx context.Context) error {
		log.Info("GetInstance called: Starting memory instance retrieval",
			"resource_id", agentMemoryRef.ID,
			"key_template", agentMemoryRef.Key,
			"resolved_key", agentMemoryRef.ResolvedKey)

		// Check for empty key template and retry
		if agentMemoryRef.Key == "" && agentMemoryRef.ResolvedKey == "" {
			workflowExecID := ExtractWorkflowExecID(workflowContextData)
			err := fmt.Errorf("memory key template is empty for resource_id=%s, workflow_exec_id=%s",
				agentMemoryRef.ID, workflowExecID)
			log.Warn("Empty key template detected, will retry",
				"resource_id", agentMemoryRef.ID,
				"workflow_exec_id", workflowExecID)
			return retry.RetryableError(err)
		}

		// If we have a key, proceed with normal flow
		var err error
		instance, err = mm.getInstanceInternal(ctx, agentMemoryRef, workflowContextData)
		if err != nil {
			return err // Non-retryable error
		}
		return nil
	})
	if err != nil {
		log.Error("All retry attempts failed for memory instance retrieval",
			"resource_id", agentMemoryRef.ID,
			"error", err)
		return nil, err
	}
	return instance, nil
}

// getInstanceInternal contains the actual instance retrieval logic
func (mm *Manager) getInstanceInternal(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (memcore.Memory, error) {
	// Load configuration and resolve key
	resourceCfg, validatedKey, projectIDVal, err := mm.prepareInstanceData(ctx, agentMemoryRef, workflowContextData)
	if err != nil {
		return nil, err
	}
	// Setup components and create instance
	return mm.createMemoryInstanceWithComponents(ctx, validatedKey, projectIDVal, resourceCfg)
}

// prepareInstanceData loads config, resolves key, and gets project ID
func (mm *Manager) prepareInstanceData(
	ctx context.Context,
	agentMemoryRef core.MemoryReference,
	workflowContextData map[string]any,
) (*memcore.Resource, string, string, error) {
	log := logger.FromContext(ctx)
	resourceCfg, err := mm.loadMemoryConfig(ctx, agentMemoryRef.ID)
	if err != nil {
		log.Error("GetInstance: Failed to load memory config", "resource_id", agentMemoryRef.ID, "error", err)
		return nil, "", "", err
	}
	validatedKey, err := mm.resolveMemoryKey(ctx, agentMemoryRef, workflowContextData)
	if err != nil {
		log.Error("GetInstance: Memory key resolution failed", "error", err)
		return nil, "", "", err
	}
	projectIDVal := mm.getProjectID(ctx, workflowContextData)
	log.Debug("GetInstance: Data preparation complete", "validated_key", validatedKey, "project_id", projectIDVal)
	return resourceCfg, validatedKey, projectIDVal, nil
}

// createMemoryInstanceWithComponents builds components and creates the memory instance
func (mm *Manager) createMemoryInstanceWithComponents(
	ctx context.Context,
	validatedKey, projectIDVal string,
	resourceCfg *memcore.Resource,
) (memcore.Memory, error) {
	log := logger.FromContext(ctx)
	components, err := mm.buildMemoryComponents(ctx, resourceCfg, projectIDVal)
	if err != nil {
		log.Error("GetInstance: Failed to build memory components", "error", err)
		return nil, err
	}
	err = mm.registerPrivacyPolicy(ctx, resourceCfg)
	if err != nil {
		log.Error("GetInstance: Failed to register privacy policy", "error", err)
		return nil, err
	}
	instance, err := mm.createMemoryInstance(ctx, validatedKey, projectIDVal, resourceCfg, components)
	if err != nil {
		log.Error("GetInstance: Failed to create memory instance", "error", err)
		return nil, err
	}
	log.Info(
		"GetInstance: Memory instance successfully created",
		"instance_id",
		validatedKey,
		"resource_id",
		resourceCfg.ID,
	)
	return instance, nil
}

// GetTokenCounter returns a new token counter.
func (mm *Manager) GetTokenCounter(_ context.Context) (memcore.TokenCounter, error) {
	return tokens.NewTiktokenCounter(DefaultTokenCounterModel)
}

// GetMemoryConfig retrieves the memory configuration for a given memory ID.
// This method is useful for retrieving configuration details like MaxTokens.
func (mm *Manager) GetMemoryConfig(ctx context.Context, memoryID string) (*memcore.Resource, error) {
	return mm.loadMemoryConfig(ctx, memoryID)
}
