package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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

// CompositeTaskMetadata represents metadata for composite tasks
type CompositeTaskMetadata struct {
	ParentStateID core.ID       `json:"parent_state_id"`
	ChildConfigs  []task.Config `json:"child_configs"`
	Strategy      string        `json:"strategy"`
	MaxWorkers    int           `json:"max_workers"`
}

// CollectionMetadata represents the result of collection processing (for state output)
type CollectionMetadata struct {
	ItemCount    int    `json:"item_count"`
	SkippedCount int    `json:"skipped_count"`
	Mode         string `json:"mode"`
	BatchSize    int    `json:"batch_size"`
}

type ConfigManager struct {
	cwd                  *core.PathCWD
	configStore          ConfigStore
	collectionNormalizer *collection.Normalizer
	contextBuilder       *shared.ContextBuilder
	configBuilder        *collection.ConfigBuilder
}

func NewConfigManager(configStore ConfigStore, cwd *core.PathCWD) (*ConfigManager, error) {
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	engine := tplengine.NewEngine(tplengine.FormatJSON)
	templateEngineAdapter := task2.NewTemplateEngineAdapter(engine)
	collectionNormalizer := collection.NewNormalizer(templateEngineAdapter, contextBuilder)
	configBuilder := collection.NewConfigBuilder(templateEngineAdapter)
	return &ConfigManager{
		cwd:                  cwd,
		configStore:          configStore,
		collectionNormalizer: collectionNormalizer,
		contextBuilder:       contextBuilder,
		configBuilder:        configBuilder,
	}, nil
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

	// Ensure child configs inherit CWD from parent before validation
	if err := task.PropagateTaskListCWD(taskConfig.Tasks, taskConfig.CWD, "parallel child task"); err != nil {
		return fmt.Errorf("failed to propagate CWD to child configs: %w", err)
	}

	// Validate child configs
	for i := range taskConfig.Tasks {
		child := &taskConfig.Tasks[i]
		if child.ID == "" {
			return fmt.Errorf("child config at index %d missing required ID field", i)
		}
		if child.CWD == nil {
			child.CWD = cm.cwd
		}
		// Perform full validation on each child config
		if err := child.Validate(); err != nil {
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
	workflowConfig *workflow.Config,
) (*CollectionMetadata, error) {
	// Validate inputs
	if err := cm.validateCollectionInputs(ctx, parentStateID, taskConfig, workflowState); err != nil {
		return nil, err
	}

	// Process collection items
	templateContext := cm.contextBuilder.BuildCollectionContext(workflowState, workflowConfig, taskConfig)
	filteredItems, skippedCount, err := cm.processCollectionItems(ctx, taskConfig, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process collection items for parent %s: %w", parentStateID, err)
	}

	// Create child configs from filtered items
	childConfigs, err := cm.createChildConfigs(taskConfig, filteredItems, templateContext)
	if err != nil {
		return nil, err
	}

	// Handle empty collections
	if len(childConfigs) == 0 {
		return cm.handleEmptyCollection(ctx, parentStateID, taskConfig, skippedCount)
	}

	// Ensure child configs inherit CWD from parent before validation
	if err := task.PropagateTaskListCWD(childConfigs, taskConfig.CWD, "collection child task"); err != nil {
		return nil, fmt.Errorf("failed to propagate CWD to child configs: %w", err)
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

// PrepareCompositeConfigs stores composite task configuration for later child creation
func (cm *ConfigManager) PrepareCompositeConfigs(
	ctx context.Context,
	parentStateID core.ID,
	taskConfig *task.Config,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if taskConfig == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if taskConfig.Type != task.TaskTypeComposite {
		return fmt.Errorf("task config must be composite type, got: %s", taskConfig.Type)
	}
	// Allow empty composite tasks - they should complete successfully with no children
	if len(taskConfig.Tasks) > 0 {
		if err := task.PropagateTaskListCWD(taskConfig.Tasks, taskConfig.CWD, "composite child task"); err != nil {
			return fmt.Errorf("failed to propagate CWD to child configs: %w", err)
		}
		// Validate child configs
		for i := range taskConfig.Tasks {
			childConfig := &taskConfig.Tasks[i]
			if childConfig.ID == "" {
				return fmt.Errorf("child config at index %d missing required ID field", i)
			}
			if childConfig.CWD == nil {
				childConfig.CWD = cm.cwd
			}
			if err := childConfig.Validate(); err != nil {
				return fmt.Errorf("invalid child config at index %d: %w", i, err)
			}
		}
	}
	metadata := &CompositeTaskMetadata{
		ParentStateID: parentStateID,
		ChildConfigs:  taskConfig.Tasks,
		Strategy:      string(task.StrategyWaitAll), // Composite tasks are always sequential
		MaxWorkers:    1,                            // Composite tasks are always sequential
	}
	key := cm.buildCompositeMetadataKey(parentStateID)
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal composite metadata for parent %s: %w", parentStateID, err)
	}
	if err := cm.configStore.SaveMetadata(ctx, key, metadataBytes); err != nil {
		return fmt.Errorf("failed to store composite metadata for parent %s: %w", parentStateID, err)
	}
	return nil
}

// LoadCompositeTaskMetadata retrieves stored metadata for composite task
func (cm *ConfigManager) LoadCompositeTaskMetadata(
	ctx context.Context,
	parentStateID core.ID,
) (*CompositeTaskMetadata, error) {
	key := cm.buildCompositeMetadataKey(parentStateID)
	metadataBytes, err := cm.configStore.GetMetadata(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get composite task metadata: %w", err)
	}
	var metadata CompositeTaskMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal composite task metadata: %w", err)
	}
	return &metadata, nil
}

func (cm *ConfigManager) buildCompositeMetadataKey(parentStateID core.ID) string {
	return fmt.Sprintf("composite_metadata:%s", parentStateID.String())
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

// validateCollectionInputs validates inputs for PrepareCollectionConfigs
func (cm *ConfigManager) validateCollectionInputs(
	_ context.Context,
	parentStateID core.ID,
	taskConfig *task.Config,
	workflowState *workflow.State,
) error {
	if parentStateID == "" {
		return fmt.Errorf("parent state ID cannot be empty")
	}
	if taskConfig == nil {
		return fmt.Errorf("task config cannot be nil")
	}
	if taskConfig.Type != task.TaskTypeCollection {
		return fmt.Errorf("task config must be collection type, got: %s", taskConfig.Type)
	}
	if workflowState == nil {
		return fmt.Errorf("workflow state cannot be nil")
	}
	return nil
}

// handleEmptyCollection handles the case when a collection has no items
func (cm *ConfigManager) handleEmptyCollection(
	ctx context.Context,
	parentStateID core.ID,
	taskConfig *task.Config,
	skippedCount int,
) (*CollectionMetadata, error) {
	metadata := &CollectionTaskMetadata{
		ParentStateID: parentStateID,
		ChildConfigs:  []task.Config{}, // Empty slice
		Strategy:      string(task.StrategyWaitAll),
		MaxWorkers:    1,
		ItemCount:     0,
		SkippedCount:  skippedCount,
		Mode:          string(taskConfig.GetMode()),
		BatchSize:     taskConfig.Batch,
	}

	key := cm.buildCollectionMetadataKey(parentStateID)
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal empty collection metadata: %w", err)
	}

	if err := cm.configStore.SaveMetadata(ctx, key, metadataBytes); err != nil {
		return nil, fmt.Errorf("failed to save empty collection metadata for parent %s: %w", parentStateID, err)
	}

	return &CollectionMetadata{
		ItemCount:    0,
		SkippedCount: skippedCount,
		Mode:         string(taskConfig.GetMode()),
		BatchSize:    taskConfig.Batch,
	}, nil
}

// createChildConfigs creates child configs from collection items
func (cm *ConfigManager) createChildConfigs(
	taskConfig *task.Config,
	filteredItems []any,
	templateContext map[string]any,
) ([]task.Config, error) {
	var childConfigs []task.Config

	for i, item := range filteredItems {
		itemContext := cm.collectionNormalizer.CreateItemContext(templateContext, &taskConfig.CollectionConfig, item, i)
		childConfig, err := cm.configBuilder.BuildTaskConfig(
			&taskConfig.CollectionConfig,
			taskConfig,
			item,
			i,
			itemContext,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create child config for item %d: %w", i, err)
		}

		// Process the With field templates using the item context
		if childConfig.With != nil {
			processedWith, err := cm.configBuilder.GetTemplateEngine().ParseValue(*childConfig.With, itemContext)
			if err != nil {
				return nil, fmt.Errorf("failed to process child config templates for item %d: %w", i, err)
			}
			if processedMap, ok := processedWith.(map[string]any); ok {
				processedInput := core.Input(processedMap)
				childConfig.With = &processedInput
			} else {
				return nil, fmt.Errorf("expected map[string]any for processed With field, got %T", processedWith)
			}
		}

		childConfigs = append(childConfigs, *childConfig)
	}

	return childConfigs, nil
}
