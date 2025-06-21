package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/CompoZy/llm-router/engine/llm"     // For llm.Message
	"github.com/CompoZy/llm-router/engine/memory/activities" // For FlushMemoryActivityInput
	"github.com/CompoZy/llm-router/pkg/logger" // Standard logger
	"go.temporal.io/sdk/client"                // For Temporal client
)

// MemoryInstance implements the Memory interface and orchestrates
// storage, locking, token management, and flushing for a single memory stream.
type MemoryInstance struct {
	id                 string // The resolved, unique key for this memory instance (e.g., "user123:chat")
	resourceID         string // The ID of the MemoryResource config that defines its behavior
	projectID          string // Project ID for namespacing, metrics, etc. (if applicable)
	resourceConfig     *Config  // Pointer to the parsed memory.Config for this instance
	store              MemoryStore
	lockManager        *MemoryLockManager // Using the wrapper from Task 1
	tokenManager       *TokenMemoryManager
	flushingStrategy   *HybridFlushingStrategy // Used to decide *if* to flush
	temporalClient     client.Client           // Temporal client to schedule activities
	temporalTaskQueue  string                  // Task queue for memory activities
	log                logger.Logger
}

// NewMemoryInstanceOptions holds options for creating a MemoryInstance.
type NewMemoryInstanceOptions struct {
	InstanceID        string // Resolved unique key for the instance
	ResourceID        string // ID of the memory.Config definition
	ProjectID         string // Optional project ID
	ResourceConfig    *Config
	Store             MemoryStore
	LockManager       *MemoryLockManager
	TokenManager      *TokenMemoryManager
	FlushingStrategy  *HybridFlushingStrategy
	TemporalClient    client.Client
	TemporalTaskQueue string
	Logger            logger.Logger
}

// NewMemoryInstance creates a new memory instance.
func NewMemoryInstance(opts NewMemoryInstanceOptions) (*MemoryInstance, error) {
	if opts.InstanceID == "" {
		return nil, fmt.Errorf("instance ID cannot be empty")
	}
	if opts.ResourceConfig == nil {
		return nil, fmt.Errorf("resource config cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("memory store cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.LockManager == nil {
		return nil, fmt.Errorf("lock manager cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TokenManager == nil {
		return nil, fmt.Errorf("token manager cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.FlushingStrategy == nil { // Even if not summarizing, strategy obj might be needed for ShouldFlush
		return nil, fmt.Errorf("flushing strategy cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TemporalClient == nil {
		return nil, fmt.Errorf("temporal client cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TemporalTaskQueue == "" {
		opts.TemporalTaskQueue = "memory-operations" // Default task queue
	}
	if opts.Logger == nil {
		opts.Logger = logger.NewNopLogger() // Default to NopLogger if none provided
	}

	return &MemoryInstance{
		id:                 opts.InstanceID,
		resourceID:         opts.ResourceID,
		projectID:          opts.ProjectID,
		resourceConfig:     opts.ResourceConfig,
		store:              opts.Store,
		lockManager:        opts.LockManager,
		tokenManager:       opts.TokenManager,
		flushingStrategy:   opts.FlushingStrategy,
		temporalClient:     opts.TemporalClient,
		temporalTaskQueue:  opts.TemporalTaskQueue,
		log:                opts.Logger.With("memory_instance_id", opts.InstanceID, "memory_resource_id", opts.ResourceID),
	}, nil
}

// GetID returns the unique identifier of this memory instance.
func (mi *MemoryInstance) GetID() string {
	return mi.id
}

// Append adds a message to the memory.
func (mi *MemoryInstance) Append(ctx context.Context, msg llm.Message) error {
	mi.log.Debug("Append called", "message_role", msg.Role)
	lockTTL := 30 * time.Second // Configurable: lock TTL for append operation
	lock, err := mi.lockManager.Acquire(ctx, mi.id, lockTTL)
	if err != nil {
		mi.log.Error("Failed to acquire lock for append", "error", err)
		return fmt.Errorf("failed to acquire lock for append on memory %s: %w", mi.id, err)
	}
	defer func() {
		if err := lock.Release(context.Background()); err != nil {
			mi.log.Error("Failed to release lock after append", "error", err)
		}
	}()

	// Append the new message
	if err := mi.store.AppendMessage(ctx, mi.id, msg); err != nil {
		mi.log.Error("Failed to append message to store", "error", err)
		return fmt.Errorf("failed to append message to store for memory %s: %w", mi.id, err)
	}

	// Asynchronously trigger a flush check if conditions might be met.
	// This is a simplified check; a more robust one might look at current counts.
	// The actual flush logic is in the Temporal activity.
	// For now, always try to schedule a flush check after an append if a flushing strategy is configured.
	// The activity itself will do the detailed check (ShouldFlush).
	if mi.resourceConfig.Flushing != nil && mi.resourceConfig.Flushing.Type != "" {
		// Read current messages to pass to ShouldFlush and then to activity if needed
		// This is inefficient to read all messages just to check.
		// TokenMemoryManager.EnforceLimits is synchronous. Flushing is async.
		// A better approach: the flush activity reads the messages.
		// Here, we just schedule it.
		workflowID := fmt.Sprintf("memory-flush-%s-%d", mi.id, time.Now().UnixNano())
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: mi.temporalTaskQueue,
			// Potentially set timeouts, retry policies for the workflow itself
		}
		flushInput := activities.FlushMemoryActivityInput{
			MemoryInstanceKey: mi.id,
			ResourceID:        mi.resourceID,
		}
		mi.log.Info("Scheduling FlushMemory workflow", "workflow_id", workflowID)
		_, err := mi.temporalClient.ExecuteWorkflow(ctx, options, "FlushMemoryWorkflow", flushInput) // Assuming "FlushMemoryWorkflow" is registered
		if err != nil {
			// Log error but don't fail the append operation itself, as flushing is background.
			mi.log.Error("Failed to schedule FlushMemory workflow", "error", err, "workflow_id", workflowID)
		}
	}

	// After appending, ensure TTL is set/refreshed according to resource config
	if mi.resourceConfig.Persistence.TTL != "" {
		ttl, parseErr := time.ParseDuration(mi.resourceConfig.Persistence.TTL)
		if parseErr == nil && ttl > 0 {
			if err := mi.store.SetExpiration(ctx, mi.id, ttl); err != nil {
				mi.log.Warn("Failed to set/refresh TTL after append", "error", err)
				// Non-fatal for append, but log it.
			}
		}
	}

	return nil
}

// Read retrieves all messages from the memory.
func (mi *MemoryInstance) Read(ctx context.Context) ([]llm.Message, error) {
	mi.log.Debug("Read called")
	// No lock needed for read by default, assuming eventual consistency is acceptable
	// or that Redis list operations are sufficiently atomic for reads.
	// If strong consistency is needed even for reads relative to writes/flushes,
	// a read lock or different strategy might be required.
	messages, err := mi.store.ReadMessages(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to read messages from store", "error", err)
		return nil, fmt.Errorf("failed to read messages from store for memory %s: %w", mi.id, err)
	}
	return messages, nil
}

// Len returns the number of messages in the memory.
func (mi *MemoryInstance) Len(ctx context.Context) (int, error) {
	mi.log.Debug("Len called")
	count, err := mi.store.CountMessages(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to count messages in store", "error", err)
		return 0, fmt.Errorf("failed to count messages in store for memory %s: %w", mi.id, err)
	}
	return count, nil
}

// GetTokenCount returns the current estimated token count of the messages in memory.
// This involves reading all messages and tokenizing them.
func (mi *MemoryInstance) GetTokenCount(ctx context.Context) (int, error) {
	mi.log.Debug("GetTokenCount called")
	messages, err := mi.Read(ctx) // Reuse Read to get messages
	if err != nil {
		return 0, fmt.Errorf("failed to read messages for token count: %w", err)
	}
	if len(messages) == 0 {
		return 0, nil
	}
	// Use the instance's TokenManager to calculate
	_, totalTokens, err := mi.tokenManager.CalculateMessagesWithTokens(ctx, messages)
	if err != nil {
		mi.log.Error("Failed to calculate tokens", "error", err)
		return 0, fmt.Errorf("failed to calculate tokens for memory %s: %w", mi.id, err)
	}
	return totalTokens, nil
}

// GetMemoryHealth returns diagnostic information about the memory instance.
func (mi *MemoryInstance) GetMemoryHealth(ctx context.Context) (*MemoryHealth, error) {
	mi.log.Debug("GetMemoryHealth called")
	count, err := mi.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count for health check: %w", err)
	}
	tokens, err := mi.GetTokenCount(ctx) // This could be expensive
	if err != nil {
		// Don't fail health check entirely, but report partial data
		mi.log.Warn("Failed to get token count for health check", "error", err)
		tokens = -1 // Indicate error or unknown
	}

	// LastFlush and FlushStrategy would require more state tracking or querying
	// For now, these are placeholders.
	health := &MemoryHealth{
		MessageCount: count,
		TokenCount:   tokens,
		// LastFlush: nil, // Need a way to get this, e.g. from Temporal activity logs or separate store field
		FlushStrategy: string(mi.resourceConfig.Flushing.Type),
	}
	return health, nil
}

// Clear removes all messages from the memory.
func (mi *MemoryInstance) Clear(ctx context.Context) error {
	mi.log.Info("Clear called, deleting all messages")
	lockTTL := 10 * time.Second
	lock, err := mi.lockManager.Acquire(ctx, mi.id, lockTTL)
	if err != nil {
		mi.log.Error("Failed to acquire lock for clear", "error", err)
		return fmt.Errorf("failed to acquire lock for clear on memory %s: %w", mi.id, err)
	}
	defer func() {
		if err := lock.Release(context.Background()); err != nil {
			mi.log.Error("Failed to release lock after clear", "error", err)
		}
	}()

	if err := mi.store.DeleteMessages(ctx, mi.id); err != nil {
		mi.log.Error("Failed to delete messages from store", "error", err)
		return fmt.Errorf("failed to delete messages from store for memory %s: %w", mi.id, err)
	}
	return nil
}

// HealthCheck performs a basic health check of the memory instance's dependencies (e.g., store).
// This is different from GetMemoryHealth which returns operational stats.
func (mi *MemoryInstance) HealthCheck(ctx context.Context) error {
	// Check store health (e.g., Redis ping if store is Redis-based)
	// This requires MemoryStore to have a HealthCheck method, or we check a raw client.
	// For now, let's assume the underlying client used by the store can be pinged.
	// This is a simplification.
	if storePinger, ok := mi.store.(interface{ Ping(context.Context) error }); ok {
		if err := storePinger.Ping(ctx); err != nil {
			return fmt.Errorf("memory store ping failed for instance %s: %w", mi.id, err)
		}
	}
	// Could also check lock manager connectivity if it has a similar ping/health method.
	mi.log.Debug("HealthCheck passed")
	return nil
}

// registerFlushWorkflow is a helper to show where workflow registration would happen.
// In a real application, this is done once when setting up Temporal workers.
func registerWorkflowsAndActivities(worker client.Worker, activityInstance *activities.MemoryActivities) {
	// worker.RegisterWorkflow(FlushMemoryWorkflow) // Definition of FlushMemoryWorkflow needed
	// worker.RegisterActivity(activityInstance.FlushMemory) // Registering the method
}

// Placeholder for the actual workflow function.
// This would typically live in a different file, e.g., memory_workflows.go
// func FlushMemoryWorkflow(ctx workflow.Context, input activities.FlushMemoryActivityInput) (*activities.FlushMemoryActivityOutput, error) {
// 	ao := workflow.ActivityOptions{
// 		StartToCloseTimeout: 10 * time.Minute, // Example timeout
// 		RetryPolicy: &temporal.RetryPolicy{
// 			MaximumAttempts: 3,
// 		},
// 	}
// 	ctx = workflow.WithActivityOptions(ctx, ao)
// 	var result activities.FlushMemoryActivityOutput
// 	err := workflow.ExecuteActivity(ctx, "FlushMemory", input).Get(ctx, &result) // "FlushMemory" is the activity func name
// 	return &result, err
// }
