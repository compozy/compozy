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
	if taskConfigs != nil {
		if taskConfig, exists := taskConfigs[taskID]; exists {
			if err := cm.MergeTaskConfig(taskContext, taskConfig); err != nil {
				// Log error but continue - best effort merge
				// This is non-critical for context building
				taskContext["_merge_error"] = err.Error()
			}
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
