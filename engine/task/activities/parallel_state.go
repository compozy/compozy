package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	taskscore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateParallelStateLabel = "CreateParallelState"

type CreateParallelStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateParallelState handles parallel state creation with tasks integration
type CreateParallelState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
	tasksFactory       tasks.Factory
	configStore        services.ConfigStore
	cwd                *core.PathCWD
}

// NewCreateParallelState creates a new CreateParallelState activity with tasks integration
func NewCreateParallelState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
	tasksFactory tasks.Factory,
) (*CreateParallelState, error) {
	return &CreateParallelState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configStore, tasksFactory, cwd),
		tasksFactory:       tasksFactory,
		configStore:        configStore,
		cwd:                cwd,
	}, nil
}

func (a *CreateParallelState) Run(ctx context.Context, input *CreateParallelStateInput) (*task.State, error) {
	if err := validateParallelInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowContext(ctx, input)
	if err != nil {
		return nil, err
	}
	state, err := a.createParentParallelState(ctx, workflowState, workflowConfig, input)
	if err != nil {
		return nil, err
	}
	normalizedConfig, err := a.normalizeParallelConfig(ctx, workflowState, workflowConfig, input.TaskConfig)
	if err != nil {
		return nil, err
	}
	childConfigs := extractParallelChildConfigs(normalizedConfig)
	if err := a.storeParallelMetadata(ctx, state, normalizedConfig, childConfigs); err != nil {
		return nil, err
	}
	a.addParallelMetadata(state, normalizedConfig, len(childConfigs))
	if err := a.createChildTasks(ctx, input, state); err != nil {
		return nil, err
	}
	return state, nil
}

// validateParallelInput ensures we only execute for valid parallel tasks.
func validateParallelInput(input *CreateParallelStateInput) error {
	if input == nil {
		return fmt.Errorf("activity input is required")
	}
	if input.TaskConfig == nil {
		return fmt.Errorf("task config is required")
	}
	if input.TaskConfig.Type != task.TaskTypeParallel {
		return fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	return nil
}

// loadWorkflowContext retrieves workflow state and config for the activity.
func (a *CreateParallelState) loadWorkflowContext(
	ctx context.Context,
	input *CreateParallelStateInput,
) (*workflow.State, *workflow.Config, error) {
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, nil, err
	}
	return workflowState, workflowConfig, nil
}

// createParentParallelState persists the parent parallel state prior to normalization.
func (a *CreateParallelState) createParentParallelState(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	input *CreateParallelStateInput,
) (*task.State, error) {
	return a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
}

// normalizeParallelConfig prepares the parallel configuration for downstream usage.
func (a *CreateParallelState) normalizeParallelConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	cfg *task.Config,
) (*task.Config, error) {
	normalizer, err := a.tasksFactory.CreateNormalizer(ctx, task.TaskTypeParallel)
	if err != nil {
		return nil, fmt.Errorf("failed to create parallel normalizer: %w", err)
	}
	normContext := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
	}
	if err := normalizer.Normalize(ctx, cfg, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize parallel task: %w", err)
	}
	return cfg, nil
}

// extractParallelChildConfigs returns pointers to normalized child configs.
func extractParallelChildConfigs(cfg *task.Config) []*task.Config {
	childConfigs := make([]*task.Config, len(cfg.Tasks))
	for i := range cfg.Tasks {
		childConfigs[i] = &cfg.Tasks[i]
	}
	return childConfigs
}

// storeParallelMetadata persists metadata required for later parallel processing.
func (a *CreateParallelState) storeParallelMetadata(
	ctx context.Context,
	state *task.State,
	cfg *task.Config,
	childConfigs []*task.Config,
) error {
	configRepo, err := a.tasksFactory.CreateTaskConfigRepository(a.configStore, a.cwd)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}
	metadata := &taskscore.ParallelTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  childConfigs,
		Strategy:      string(cfg.GetStrategy()),
		MaxWorkers:    cfg.GetMaxWorkers(),
	}
	if err := configRepo.StoreParallelMetadata(ctx, state.TaskExecID, metadata); err != nil {
		return fmt.Errorf("failed to store parallel metadata: %w", err)
	}
	return nil
}

// createChildTasks triggers creation of child tasks for the parallel parent.
func (a *CreateParallelState) createChildTasks(
	ctx context.Context,
	input *CreateParallelStateInput,
	state *task.State,
) error {
	if err := a.createChildTasksUC.Execute(ctx, &uc.CreateChildTasksInput{
		ParentStateID:  state.TaskExecID,
		WorkflowExecID: input.WorkflowExecID,
		WorkflowID:     input.WorkflowID,
	}); err != nil {
		return fmt.Errorf("failed to create child tasks: %w", err)
	}
	return nil
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
