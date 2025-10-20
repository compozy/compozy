package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateParallelStateLabel = "CreateParallelState"

type CreateParallelStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateParallelState handles parallel state creation with task2 integration
type CreateParallelState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
	task2Factory       task2.Factory
	configStore        services.ConfigStore
	cwd                *core.PathCWD
}

// NewCreateParallelState creates a new CreateParallelState activity with task2 integration
func NewCreateParallelState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
	task2Factory task2.Factory,
) (*CreateParallelState, error) {
	return &CreateParallelState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configStore, task2Factory, cwd),
		task2Factory:       task2Factory,
		configStore:        configStore,
		cwd:                cwd,
	}, nil
}

func (a *CreateParallelState) Run(ctx context.Context, input *CreateParallelStateInput) (*task.State, error) {
	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeParallel {
		return nil, fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	// Load workflow context
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	// Create parent state first with the original parallel config
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 normalizer to prepare parallel task configs
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeParallel)
	if err != nil {
		return nil, fmt.Errorf("failed to create parallel normalizer: %w", err)
	}
	// Create normalization context
	normContext := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
	}
	// Normalize the parallel task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(ctx, normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize parallel task: %w", err)
	}
	// Get child configs from normalized parallel config
	childConfigs := make([]*task.Config, len(normalizedConfig.Tasks))
	for i := range normalizedConfig.Tasks {
		childConfigs[i] = &normalizedConfig.Tasks[i]
	}
	// Store parallel metadata using task2 repository
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config repository: %w", err)
	}
	parallelMetadata := &task2core.ParallelTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  childConfigs,
		Strategy:      string(normalizedConfig.GetStrategy()),
		MaxWorkers:    normalizedConfig.GetMaxWorkers(),
	}
	if err := configRepo.StoreParallelMetadata(ctx, state.TaskExecID, parallelMetadata); err != nil {
		return nil, fmt.Errorf("failed to store parallel metadata: %w", err)
	}
	// Add metadata to state output
	a.addParallelMetadata(state, normalizedConfig, len(childConfigs))
	// Create child tasks
	if err := a.createChildTasksUC.Execute(ctx, &uc.CreateChildTasksInput{
		ParentStateID:  state.TaskExecID,
		WorkflowExecID: input.WorkflowExecID,
		WorkflowID:     input.WorkflowID,
	}); err != nil {
		return nil, fmt.Errorf("failed to create child tasks: %w", err)
	}
	return state, nil
}

func (a *CreateParallelState) addParallelMetadata(state *task.State, config *task.Config, childCount int) {
	if state.Output == nil {
		output := make(core.Output)
		state.Output = &output
	}
	(*state.Output)["parallel_metadata"] = map[string]any{
		"child_count": childCount,
		"strategy":    string(config.GetStrategy()),
		"max_workers": config.GetMaxWorkers(),
	}
}
