package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
)

// ConfigMerger is responsible for merging task configurations
type ConfigMerger struct{}

// NewConfigMerger creates a new config merger
func NewConfigMerger() *ConfigMerger {
	return &ConfigMerger{}
}

// MergeTaskConfigIfExists merges task config into task context
func (cm *ConfigMerger) MergeTaskConfigIfExists(
	taskContext map[string]any,
	taskID string,
	taskConfigs map[string]*task.Config,
) {
	if taskContext == nil {
		return
	}
	if taskConfigs != nil {
		if taskConfig, exists := taskConfigs[taskID]; exists {
			// Ignore merge errors - best effort merge for non-critical context building
			//nolint:errcheck // Intentionally ignoring errors for non-critical merge operation
			_ = cm.MergeTaskConfig(taskContext, taskConfig)
		}
	}
}

// MergeTaskConfig merges task configuration
func (cm *ConfigMerger) MergeTaskConfig(taskContext map[string]any, taskConfig *task.Config) error {
	taskConfigMap, err := taskConfig.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	for k, v := range taskConfigMap {
		if k != "input" && k != "output" { // Don't override runtime state
			taskContext[k] = v
		}
	}
	return nil
}
