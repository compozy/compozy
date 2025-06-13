package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

// ParallelTaskMetadata represents metadata for parallel tasks
type ParallelTaskMetadata struct {
	ParentStateID core.ID       `json:"parent_state_id"`
	ChildConfigs  []task.Config `json:"child_configs"`
	Strategy      string        `json:"strategy"`
	MaxWorkers    int           `json:"max_workers"`
}

// CollectionTaskMetadata represents metadata for collection tasks
type CollectionTaskMetadata struct {
	ParentStateID core.ID       `json:"parent_state_id"`
	ChildConfigs  []task.Config `json:"child_configs"`
	Strategy      string        `json:"strategy"`
	MaxWorkers    int           `json:"max_workers"`
	ItemCount     int           `json:"item_count"`
	SkippedCount  int           `json:"skipped_count"`
	Mode          string        `json:"mode"`
	BatchSize     int           `json:"batch_size"`
}

// CollectionMetadata represents the result of collection processing (for state output)
type CollectionMetadata struct {
	ItemCount    int    `json:"item_count"`
	SkippedCount int    `json:"skipped_count"`
	Mode         string `json:"mode"`
	BatchSize    int    `json:"batch_size"`
}

type ConfigManager struct {
	configStore          ConfigStore
	collectionNormalizer *normalizer.CollectionNormalizer
	configBuilder        *normalizer.CollectionConfigBuilder
	contextBuilder       *normalizer.ContextBuilder
}

func NewConfigManager(configStore ConfigStore) *ConfigManager {
	return &ConfigManager{
		configStore:          configStore,
		collectionNormalizer: normalizer.NewCollectionNormalizer(),
		configBuilder:        normalizer.NewCollectionConfigBuilder(),
		contextBuilder:       normalizer.NewContextBuilder(),
	}
}

// PrepareParallelConfigs stores parallel task configuration for later child creation
func (cm *ConfigManager) PrepareParallelConfigs(
	ctx context.Context,
	parentStateID core.ID,
	taskConfig *task.Config,
) error {
	// Defensive validation
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if taskConfig == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if taskConfig.Type != task.TaskTypeParallel {
		return fmt.Errorf("task config must be parallel type, got: %s", taskConfig.Type)
	}
	if len(taskConfig.Tasks) == 0 {
		return fmt.Errorf("parallel task must have at least one child task")
	}

	// Validate child configs
	for i := range taskConfig.Tasks {
		if taskConfig.Tasks[i].ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
		// Perform full validation on each child config
		if err := taskConfig.Tasks[i].Validate(); err != nil {
			return fmt.Errorf("invalid child config at index %d: %w", i, err)
		}
	}

	metadata := &ParallelTaskMetadata{
		ParentStateID: parentStateID,
		ChildConfigs:  taskConfig.Tasks,
		Strategy:      string(taskConfig.GetStrategy()),
		MaxWorkers:    taskConfig.GetMaxWorkers(),
	}

	key := cm.buildParallelMetadataKey(parentStateID)
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal parallel metadata for parent %s: %w", parentStateID, err)
	}

	if err := cm.configStore.SaveMetadata(ctx, key, metadataBytes); err != nil {
		return fmt.Errorf("failed to save parallel metadata for parent %s: %w", parentStateID, err)
	}

	return nil
}

// PrepareCollectionConfigs processes collection items and stores child configs
func (cm *ConfigManager) PrepareCollectionConfigs(
	ctx context.Context,
	parentStateID core.ID,
	taskConfig *task.Config,
	workflowState *workflow.State,
) (*CollectionMetadata, error) {
	// Defensive validation
	if parentStateID == "" {
		return nil, fmt.Errorf("parent state ID cannot be empty")
	}
	if taskConfig == nil {
		return nil, fmt.Errorf("task config cannot be nil")
	}
	if taskConfig.Type != task.TaskTypeCollection {
		return nil, fmt.Errorf("task config must be collection type, got: %s", taskConfig.Type)
	}
	if workflowState == nil {
		return nil, fmt.Errorf("workflow state cannot be nil")
	}

	// Process collection items
	templateContext := cm.contextBuilder.BuildCollectionContext(workflowState, taskConfig)
	filteredItems, skippedCount, err := cm.processCollectionItems(ctx, taskConfig, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process collection items for parent %s: %w", parentStateID, err)
	}

	// Create child configs
	childConfigs, err := cm.configBuilder.CreateChildConfigs(taskConfig, filteredItems, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create child configs for parent %s: %w", parentStateID, err)
	}

	// Validate generated child configs
	if len(childConfigs) == 0 {
		return nil, fmt.Errorf("no child configs generated for collection task %s", parentStateID)
	}
	for i := range childConfigs {
		if childConfigs[i].ID == "" {
			return nil, fmt.Errorf("generated child config at index %d missing required ID field", i)
		}
		// Perform full validation on each generated child config
		if err := childConfigs[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid generated child config at index %d: %w", i, err)
		}
	}

	metadata := &CollectionTaskMetadata{
		ParentStateID: parentStateID,
		ChildConfigs:  childConfigs,
		Strategy:      string(task.StrategyWaitAll), // Collections use wait_all
		MaxWorkers:    cm.calculateMaxWorkers(taskConfig),
		ItemCount:     len(filteredItems),
		SkippedCount:  skippedCount,
		Mode:          string(taskConfig.GetMode()),
		BatchSize:     taskConfig.Batch,
	}

	key := cm.buildCollectionMetadataKey(parentStateID)
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal collection metadata: %w", err)
	}

	if err := cm.configStore.SaveMetadata(ctx, key, metadataBytes); err != nil {
		return nil, fmt.Errorf("failed to save collection metadata for parent %s: %w", parentStateID, err)
	}

	return &CollectionMetadata{
		ItemCount:    len(filteredItems),
		SkippedCount: skippedCount,
		Mode:         string(taskConfig.GetMode()),
		BatchSize:    taskConfig.Batch,
	}, nil
}

// LoadParallelTaskMetadata retrieves stored metadata for parallel task
func (cm *ConfigManager) LoadParallelTaskMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (*ParallelTaskMetadata, error) {
	key := cm.buildParallelMetadataKey(parentStateID)
	metadataBytes, err := cm.configStore.GetMetadata(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get parallel task metadata: %w", err)
	}

	var metadata ParallelTaskMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parallel task metadata: %w", err)
	}
	return &metadata, nil
}

// LoadCollectionTaskMetadata retrieves stored metadata for collection task
func (cm *ConfigManager) LoadCollectionTaskMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (*CollectionTaskMetadata, error) {
	key := cm.buildCollectionMetadataKey(parentStateID)
	metadataBytes, err := cm.configStore.GetMetadata(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection task metadata: %w", err)
	}

	var metadata CollectionTaskMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal collection task metadata: %w", err)
	}
	return &metadata, nil
}

func (cm *ConfigManager) buildParallelMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("parallel_metadata:%s", parentStateID.String())
}

func (cm *ConfigManager) buildCollectionMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("collection_metadata:%s", parentStateID.String())
}

// processCollectionItems expands and filters collection items using the normalizer
func (cm *ConfigManager) processCollectionItems(
	ctx context.Context,
	taskConfig *task.Config,
	templateContext map[string]any,
) ([]any, int, error) {
	items, err := cm.collectionNormalizer.ExpandCollectionItems(ctx, &taskConfig.CollectionConfig, templateContext)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to expand collection items: %w", err)
	}

	filteredItems, err := cm.collectionNormalizer.FilterCollectionItems(
		ctx,
		&taskConfig.CollectionConfig,
		items,
		templateContext,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to filter collection items: %w", err)
	}

	skippedCount := len(items) - len(filteredItems)
	return filteredItems, skippedCount, nil
}

func (cm *ConfigManager) calculateMaxWorkers(taskConfig *task.Config) int {
	if taskConfig.GetMode() == task.CollectionModeSequential {
		if taskConfig.Batch > 0 {
			return taskConfig.Batch
		}
		return 1
	}
	return 0 // 0 means unlimited for parallel mode
}

// SaveTaskConfig saves a task configuration to Redis using taskExecID as key
func (cm *ConfigManager) SaveTaskConfig(ctx context.Context, taskExecID core.ID, config *task.Config) error {
	return cm.configStore.Save(ctx, taskExecID.String(), config)
}

// DeleteTaskConfig removes a task configuration from Redis using taskExecID as key
func (cm *ConfigManager) DeleteTaskConfig(ctx context.Context, taskExecID core.ID) error {
	return cm.configStore.Delete(ctx, taskExecID.String())
}
