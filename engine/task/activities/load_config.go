package activities

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	wf "github.com/compozy/compozy/engine/workflow"
)

const (
	LoadTaskConfigLabel   = "LoadTaskConfig"
	LoadBatchConfigsLabel = "LoadBatchConfigs"
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
