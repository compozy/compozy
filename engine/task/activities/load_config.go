package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	wf "github.com/compozy/compozy/engine/workflow"
)

const (
	LoadTaskConfigLabel       = "LoadTaskConfig"
	LoadBatchConfigsLabel     = "LoadBatchConfigs"
	LoadCompositeConfigsLabel = "LoadCompositeConfigs"
)

// LoadTaskConfigInput represents input for deterministic task config loading
type LoadTaskConfigInput struct {
	WorkflowConfig *wf.Config `json:"workflow_config"`
	TaskID         string     `json:"task_id"`
}

// LoadBatchConfigsInput represents input for batch loading task configs
type LoadBatchConfigsInput struct {
	WorkflowConfig *wf.Config `json:"workflow_config"`
	TaskIDs        []string   `json:"task_ids"`
}

// LoadCompositeConfigsInput represents input for loading composite child configs from metadata
type LoadCompositeConfigsInput struct {
	ParentTaskExecID core.ID  `json:"parent_task_exec_id"`
	TaskIDs          []string `json:"task_ids"`
}

// LoadTaskConfig activity implementation
type LoadTaskConfig struct {
	loadTaskConfigUC *uc.LoadTaskConfig
}

// NewLoadTaskConfig creates a new LoadTaskConfig activity
func NewLoadTaskConfig(workflows []*wf.Config) *LoadTaskConfig {
	return &LoadTaskConfig{
		loadTaskConfigUC: uc.NewLoadTaskConfig(workflows),
	}
}

func (a *LoadTaskConfig) Run(_ context.Context, input *LoadTaskConfigInput) (*task.Config, error) {
	// Note: LoadTaskConfig UC ignores context (passed as _) and only does in-memory lookups
	return a.loadTaskConfigUC.Execute(nil, &uc.LoadTaskConfigInput{
		WorkflowConfig: input.WorkflowConfig,
		TaskID:         input.TaskID,
	})
}

// LoadBatchConfigs activity implementation
type LoadBatchConfigs struct {
	loadTaskConfigUC *uc.LoadTaskConfig
}

// NewLoadBatchConfigs creates a new LoadBatchConfigs activity
func NewLoadBatchConfigs(workflows []*wf.Config) *LoadBatchConfigs {
	return &LoadBatchConfigs{
		loadTaskConfigUC: uc.NewLoadTaskConfig(workflows),
	}
}

func (a *LoadBatchConfigs) Run(_ context.Context, input *LoadBatchConfigsInput) (map[string]*task.Config, error) {
	configs := make(map[string]*task.Config, len(input.TaskIDs))
	for _, taskID := range input.TaskIDs {
		// Note: LoadTaskConfig UC ignores context (passed as _) and only does in-memory lookups
		config, err := a.loadTaskConfigUC.Execute(nil, &uc.LoadTaskConfigInput{
			WorkflowConfig: input.WorkflowConfig,
			TaskID:         taskID,
		})
		if err != nil {
			return nil, err
		}
		configs[taskID] = config
	}

	return configs, nil
}

// LoadCompositeConfigs activity implementation
type LoadCompositeConfigs struct {
	configManager *services.ConfigManager
}

// NewLoadCompositeConfigs creates a new LoadCompositeConfigs activity
func NewLoadCompositeConfigs(configManager *services.ConfigManager) *LoadCompositeConfigs {
	return &LoadCompositeConfigs{
		configManager: configManager,
	}
}

func (a *LoadCompositeConfigs) Run(
	ctx context.Context,
	input *LoadCompositeConfigsInput,
) (map[string]*task.Config, error) {
	// For composite tasks, child configs are stored individually by their TaskExecID
	// We need to map TaskID -> TaskExecID, then load each config
	// First, get the composite metadata to find child TaskExecIDs
	metadata, err := a.configManager.LoadCompositeTaskMetadata(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load composite metadata: %w", err)
	}
	// Create TaskID -> Config mapping for requested tasks
	configs := make(map[string]*task.Config, len(input.TaskIDs))
	// Optimize: Build a map of available child configs first
	childConfigsByID := make(map[string]*task.Config)
	for i := range metadata.ChildConfigs {
		childConfigsByID[metadata.ChildConfigs[i].ID] = &metadata.ChildConfigs[i]
	}
	// Return only the requested configs (memory efficient for large composite tasks)
	for _, taskID := range input.TaskIDs {
		config, exists := childConfigsByID[taskID]
		if !exists {
			availableIDs := getChildConfigIDs(metadata.ChildConfigs)
			const maxIDsToShow = 20
			displayIDs := availableIDs
			if len(availableIDs) > maxIDsToShow {
				displayIDs = availableIDs[:maxIDsToShow]
			}
			return nil, fmt.Errorf(
				"child config not found: task_id=%s, parent_exec_id=%s, available_configs_count=%d, available_configs_sample=%v",
				taskID,
				input.ParentTaskExecID,
				len(availableIDs),
				displayIDs,
			)
		}
		configs[taskID] = config
	}
	return configs, nil
}

func getChildConfigIDs(configs []task.Config) []string {
	ids := make([]string, len(configs))
	for i := range configs {
		ids[i] = configs[i].ID
	}
	return ids
}
