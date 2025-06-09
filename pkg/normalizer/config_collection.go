package normalizer

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// CollectionConfigBuilder handles creation of child configurations for collection tasks
type CollectionConfigBuilder struct {
	collectionNormalizer *CollectionNormalizer
}

// NewCollectionConfigBuilder creates a new collection config builder
func NewCollectionConfigBuilder() *CollectionConfigBuilder {
	return &CollectionConfigBuilder{
		collectionNormalizer: NewCollectionNormalizer(),
	}
}

// CreateChildConfigs creates task configurations for each filtered item
func (ccb *CollectionConfigBuilder) CreateChildConfigs(
	taskConfig *task.Config,
	filteredItems []any,
	templateContext map[string]any,
) ([]task.Config, error) {
	switch {
	case taskConfig.Task != nil:
		return ccb.createConfigsFromTaskTemplate(taskConfig, filteredItems, templateContext)
	case len(taskConfig.Tasks) > 0:
		return ccb.createConfigsFromTasksArray(taskConfig, filteredItems, templateContext)
	default:
		return nil, fmt.Errorf("collection task must have either a task template or tasks array")
	}
}

// createConfigsFromTaskTemplate creates configs using a single task template
func (ccb *CollectionConfigBuilder) createConfigsFromTaskTemplate(
	taskConfig *task.Config,
	filteredItems []any,
	templateContext map[string]any,
) ([]task.Config, error) {
	var childConfigs []task.Config
	for i, item := range filteredItems {
		itemContext := ccb.collectionNormalizer.CreateItemContext(
			templateContext,
			&taskConfig.CollectionConfig,
			item,
			i,
		)
		childConfigPtr, err := deepCopyConfig(taskConfig.Task)
		if err != nil {
			return nil, fmt.Errorf("failed to deep copy task config for item %d: %w", i, err)
		}
		childConfig := *childConfigPtr
		childConfig.ID = fmt.Sprintf("%s_item_%d", taskConfig.ID, i)
		if err := ccb.collectionNormalizer.ApplyTemplateToConfig(&childConfig, itemContext); err != nil {
			return nil, fmt.Errorf("failed to apply template to task config for item %d: %w", i, err)
		}
		childConfigs = append(childConfigs, childConfig)
	}
	return childConfigs, nil
}

// createConfigsFromTasksArray creates configs using tasks array
func (ccb *CollectionConfigBuilder) createConfigsFromTasksArray(
	taskConfig *task.Config,
	filteredItems []any,
	templateContext map[string]any,
) ([]task.Config, error) {
	var childConfigs []task.Config
	for i, item := range filteredItems {
		itemContext := ccb.collectionNormalizer.CreateItemContext(
			templateContext,
			&taskConfig.CollectionConfig,
			item,
			i,
		)
		for j := range taskConfig.Tasks {
			childConfigPtr, err := deepCopyConfig(&taskConfig.Tasks[j])
			if err != nil {
				return nil, fmt.Errorf("failed to deep copy task config for item %d, task %d: %w", i, j, err)
			}
			childConfig := *childConfigPtr
			childConfig.ID = fmt.Sprintf("%s_item_%d_task_%d", taskConfig.ID, i, j)
			if err := ccb.collectionNormalizer.ApplyTemplateToConfig(&childConfig, itemContext); err != nil {
				return nil, fmt.Errorf("failed to apply template to task config for item %d, task %d: %w", i, j, err)
			}
			childConfigs = append(childConfigs, childConfig)
		}
	}
	return childConfigs, nil
}

// deepCopyConfig creates a deep copy of a task.Config using mergo to ensure isolation between iterations
func deepCopyConfig(original *task.Config) (*task.Config, error) {
	copied := task.Config{}
	if err := mergo.Merge(&copied, original, mergo.WithAppendSlice, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("failed to deep copy config: %w", err)
	}
	// Manually deep copy map pointers to ensure true isolation
	if original.With != nil {
		withCopy := core.Input(make(map[string]any))
		maps.Copy(withCopy, *original.With)
		copied.With = &withCopy
	}
	if original.Env != nil {
		envCopy := core.EnvMap(make(map[string]string))
		maps.Copy(envCopy, *original.Env)
		copied.Env = &envCopy
	}
	if original.Outputs != nil {
		outputsCopy := core.Input(make(map[string]any))
		maps.Copy(outputsCopy, *original.Outputs)
		copied.Outputs = &outputsCopy
	}
	return &copied, nil
}
