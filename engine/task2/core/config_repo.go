package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// -----------------------------------------------------------------------------
// ConfigStore Interface
// -----------------------------------------------------------------------------

// ConfigStore provides persistent storage for task configurations
// keyed by their TaskExecID. This allows workers to avoid shipping
// large config objects through Temporal history and enables
// retrieval of generated child configs for collection/parallel tasks.
type ConfigStore interface {
	// Save persists a task configuration with the given taskExecID as key
	Save(ctx context.Context, taskExecID string, config *task.Config) error
	// Get retrieves a task configuration by taskExecID
	Get(ctx context.Context, taskExecID string) (*task.Config, error)
	// Delete removes a task configuration by taskExecID
	// This can be called when a task reaches terminal status to save space
	Delete(ctx context.Context, taskExecID string) error
	// SaveMetadata persists arbitrary metadata with the given key
	SaveMetadata(ctx context.Context, key string, data []byte) error
	// GetMetadata retrieves metadata by key
	GetMetadata(ctx context.Context, key string) ([]byte, error)
	// DeleteMetadata removes metadata by key
	DeleteMetadata(ctx context.Context, key string) error
	// Close closes the underlying storage and releases resources
	Close() error
}

// -----------------------------------------------------------------------------
// Metadata Types
// -----------------------------------------------------------------------------

// ParallelTaskMetadata represents metadata for parallel task execution
type ParallelTaskMetadata struct {
	ParentStateID core.ID        `json:"parent_state_id"`
	ChildConfigs  []*task.Config `json:"child_configs"`
	Strategy      string         `json:"strategy"`
	MaxWorkers    int            `json:"max_workers"`
}

// CollectionTaskMetadata represents metadata for collection task execution
type CollectionTaskMetadata struct {
	ParentStateID core.ID        `json:"parent_state_id"`
	ChildConfigs  []*task.Config `json:"child_configs"`
	Strategy      string         `json:"strategy"`
	MaxWorkers    int            `json:"max_workers"`
	ItemCount     int            `json:"item_count"`
	SkippedCount  int            `json:"skipped_count"`
	Mode          string         `json:"mode"`
	BatchSize     int            `json:"batch_size"`
}

// CompositeTaskMetadata represents metadata for composite task execution
type CompositeTaskMetadata struct {
	ParentStateID core.ID        `json:"parent_state_id"`
	ChildConfigs  []*task.Config `json:"child_configs"`
	Strategy      string         `json:"strategy"`
	MaxWorkers    int            `json:"max_workers"`
}

// TaskConfigRepository handles storage and retrieval of task configuration data
type TaskConfigRepository struct {
	configStore ConfigStore
	cwd         *core.PathCWD
}

// NewTaskConfigRepository creates a new task configuration repository
func NewTaskConfigRepository(configStore ConfigStore, cwd *core.PathCWD) *TaskConfigRepository {
	return &TaskConfigRepository{
		configStore: configStore,
		cwd:         cwd,
	}
}

// -----------------------------------------------------------------------------
// Parallel Task Methods
// -----------------------------------------------------------------------------

// StoreParallelMetadata stores parallel task metadata with CWD propagation
func (r *TaskConfigRepository) StoreParallelMetadata(
	ctx context.Context,
	parentStateID core.ID,
	metadata any,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if metadata == nil {
		return fmt.Errorf("parallel metadata cannot be nil")
	}
	parallelMetadata, ok := metadata.(*ParallelTaskMetadata)
	if !ok {
		return fmt.Errorf("metadata must be of type *ParallelTaskMetadata")
	}
	if parallelMetadata.Strategy != "" && !r.isValidStrategy(parallelMetadata.Strategy) {
		return fmt.Errorf("invalid parallel strategy: %s", parallelMetadata.Strategy)
	}
	return r.storeMetadata(ctx, parentStateID, parallelMetadata, "parallel", parallelMetadata.ChildConfigs)
}

// LoadParallelMetadata loads parallel task metadata
func (r *TaskConfigRepository) LoadParallelMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (any, error) {
	var metadata ParallelTaskMetadata
	if err := r.loadMetadata(ctx, parentStateID, &metadata, "parallel"); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// buildParallelMetadataKey creates a consistent key for parallel metadata
func (r *TaskConfigRepository) buildParallelMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("parallel_metadata:%s", parentStateID.String())
}

// -----------------------------------------------------------------------------
// Collection Task Methods
// -----------------------------------------------------------------------------

// StoreCollectionMetadata stores collection task metadata with CWD propagation
func (r *TaskConfigRepository) StoreCollectionMetadata(
	ctx context.Context,
	parentStateID core.ID,
	metadata any,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if metadata == nil {
		return fmt.Errorf("collection metadata cannot be nil")
	}
	collectionMetadata, ok := metadata.(*CollectionTaskMetadata)
	if !ok {
		return fmt.Errorf("metadata must be of type *CollectionTaskMetadata")
	}
	if collectionMetadata.Strategy != "" && !r.isValidStrategy(collectionMetadata.Strategy) {
		return fmt.Errorf("invalid collection strategy: %s", collectionMetadata.Strategy)
	}
	return r.storeMetadata(ctx, parentStateID, collectionMetadata, "collection", collectionMetadata.ChildConfigs)
}

// LoadCollectionMetadata loads collection task metadata
func (r *TaskConfigRepository) LoadCollectionMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (any, error) {
	var metadata CollectionTaskMetadata
	if err := r.loadMetadata(ctx, parentStateID, &metadata, "collection"); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// buildCollectionMetadataKey creates a consistent key for collection metadata
func (r *TaskConfigRepository) buildCollectionMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("collection_metadata:%s", parentStateID.String())
}

// -----------------------------------------------------------------------------
// Composite Task Methods
// -----------------------------------------------------------------------------

// StoreCompositeMetadata stores composite task metadata with CWD propagation
func (r *TaskConfigRepository) StoreCompositeMetadata(
	ctx context.Context,
	parentStateID core.ID,
	metadata any,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if metadata == nil {
		return fmt.Errorf("composite metadata cannot be nil")
	}
	compositeMetadata, ok := metadata.(*CompositeTaskMetadata)
	if !ok {
		return fmt.Errorf("metadata must be of type *CompositeTaskMetadata")
	}
	if compositeMetadata.Strategy != "" && !r.isValidStrategy(compositeMetadata.Strategy) {
		return fmt.Errorf("invalid composite strategy: %s", compositeMetadata.Strategy)
	}
	return r.storeMetadata(ctx, parentStateID, compositeMetadata, "composite", compositeMetadata.ChildConfigs)
}

// LoadCompositeMetadata loads composite task metadata
func (r *TaskConfigRepository) LoadCompositeMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (any, error) {
	var metadata CompositeTaskMetadata
	if err := r.loadMetadata(ctx, parentStateID, &metadata, "composite"); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// buildCompositeMetadataKey creates a consistent key for composite metadata
func (r *TaskConfigRepository) buildCompositeMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("composite_metadata:%s", parentStateID.String())
}

// -----------------------------------------------------------------------------
// Generic Task Config Methods
// -----------------------------------------------------------------------------

// SaveTaskConfig saves a task configuration
func (r *TaskConfigRepository) SaveTaskConfig(ctx context.Context, taskExecID string, config *task.Config) error {
	if err := r.validateTaskConfigInput(taskExecID, config); err != nil {
		return err
	}
	if err := r.configStore.Save(ctx, taskExecID, config); err != nil {
		return fmt.Errorf("failed to save task config for %s: %w", taskExecID, err)
	}
	return nil
}

// GetTaskConfig retrieves a task configuration
func (r *TaskConfigRepository) GetTaskConfig(ctx context.Context, taskExecID string) (*task.Config, error) {
	if taskExecID == "" {
		return nil, fmt.Errorf("task execution ID cannot be empty")
	}
	config, err := r.configStore.Get(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task config for %s: %w", taskExecID, err)
	}
	return config, nil
}

// DeleteTaskConfig deletes a task configuration
func (r *TaskConfigRepository) DeleteTaskConfig(ctx context.Context, taskExecID string) error {
	if taskExecID == "" {
		return fmt.Errorf("task execution ID cannot be empty")
	}
	if err := r.configStore.Delete(ctx, taskExecID); err != nil {
		return fmt.Errorf("failed to delete task config for %s: %w", taskExecID, err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Strategy Management Methods
// -----------------------------------------------------------------------------

// ParallelConfigData represents parallel configuration for strategy extraction
type ParallelConfigData struct {
	Strategy   task.ParallelStrategy `json:"strategy"`
	MaxWorkers int                   `json:"max_workers"`
}

// ExtractParallelStrategy extracts parallel strategy from task state with graceful degradation
func (r *TaskConfigRepository) ExtractParallelStrategy(
	_ context.Context,
	parentState *task.State,
) (task.ParallelStrategy, error) {
	const defaultStrategy = task.StrategyWaitAll
	if parentState == nil || parentState.Input == nil {
		return defaultStrategy, nil
	}
	var configData ParallelConfigData
	switch v := (*parentState.Input)["parallel_config"].(type) {
	case map[string]any:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return defaultStrategy, nil // Graceful degradation
		}
		if err := json.Unmarshal(jsonBytes, &configData); err != nil {
			return defaultStrategy, nil // Graceful degradation
		}
	case string:
		if err := json.Unmarshal([]byte(v), &configData); err != nil {
			return defaultStrategy, nil // Graceful degradation
		}
	default:
		return defaultStrategy, nil
	}
	if !r.isValidStrategy(string(configData.Strategy)) {
		return defaultStrategy, nil
	}
	return configData.Strategy, nil
}

// ValidateStrategy validates and normalizes a strategy string
func (r *TaskConfigRepository) ValidateStrategy(strategy string) (task.ParallelStrategy, error) {
	if !r.isValidStrategy(strategy) {
		return "", fmt.Errorf("invalid parallel strategy: %s", strategy)
	}
	return task.ParallelStrategy(strategy), nil
}

// CalculateMaxWorkers calculates worker count based on task type and configuration
func (r *TaskConfigRepository) CalculateMaxWorkers(taskType task.Type, maxWorkers int) int {
	switch taskType {
	case task.TaskTypeCollection:
		if maxWorkers <= 0 {
			return 10 // Default for collections
		}
		return maxWorkers
	case task.TaskTypeParallel:
		if maxWorkers <= 0 {
			return 4 // Default for parallel
		}
		return maxWorkers
	case task.TaskTypeComposite:
		return 1
	default:
		return 1
	}
}

// isValidStrategy checks if strategy is valid
func (r *TaskConfigRepository) isValidStrategy(strategy string) bool {
	return task.ValidateStrategy(strategy)
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

// propagateCWDToChildren propagates working directory to child configs
func (r *TaskConfigRepository) propagateCWDToChildren(childConfigs []*task.Config) {
	if r.cwd == nil {
		return // No CWD to propagate
	}
	for _, childConfig := range childConfigs {
		if childConfig.CWD == nil {
			childConfig.CWD = r.cwd
		}
	}
}

// storeMetadata is a generic helper for storing metadata with consistent validation and error handling
func (r *TaskConfigRepository) storeMetadata(
	ctx context.Context,
	parentStateID core.ID,
	metadata any,
	metadataType string,
	childConfigs []*task.Config,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	r.propagateCWDToChildren(childConfigs)
	key := fmt.Sprintf("%s_metadata:%s", metadataType, parentStateID.String())
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal %s metadata for parent %s: %w", metadataType, parentStateID, err)
	}
	if err := r.configStore.SaveMetadata(ctx, key, metadataBytes); err != nil {
		return fmt.Errorf("failed to save %s metadata for parent %s: %w", metadataType, parentStateID, err)
	}
	return nil
}

// loadMetadata is a generic helper for loading metadata with consistent validation and error handling
func (r *TaskConfigRepository) loadMetadata(
	ctx context.Context,
	parentStateID core.ID,
	result any,
	metadataType string,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	key := fmt.Sprintf("%s_metadata:%s", metadataType, parentStateID.String())
	metadataBytes, err := r.configStore.GetMetadata(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get %s task metadata for parent %s: %w", metadataType, parentStateID, err)
	}
	if err := json.Unmarshal(metadataBytes, result); err != nil {
		return fmt.Errorf("failed to unmarshal %s task metadata for parent %s: %w", metadataType, parentStateID, err)
	}
	return nil
}

// validateTaskConfigInput validates task configuration input parameters
func (r *TaskConfigRepository) validateTaskConfigInput(taskExecID string, config *task.Config) error {
	if taskExecID == "" {
		return fmt.Errorf("task execution ID cannot be empty")
	}
	if config == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if config.ID == "" {
		return fmt.Errorf("task config ID cannot be empty")
	}
	if config.Type == "" {
		return fmt.Errorf("task config type cannot be empty")
	}
	return nil
}
