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

const CreateCompositeStateLabel = "CreateCompositeState"
const compositeMaxWorkersSingle = 1

type CreateCompositeStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateCompositeState handles composite state creation with task2 integration
type CreateCompositeState struct {
	loadWorkflowUC *uc.LoadWorkflow
	taskRepo       task.Repository
	task2Factory   task2.Factory
	configStore    services.ConfigStore
	cwd            *core.PathCWD
}

// NewCreateCompositeState creates a new CreateCompositeState activity with task2 integration
func NewCreateCompositeState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
	task2Factory task2.Factory,
) (*CreateCompositeState, error) {
	return &CreateCompositeState{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		taskRepo:       taskRepo,
		task2Factory:   task2Factory,
		configStore:    configStore,
		cwd:            cwd,
	}, nil
}

func validateCompositeStateInput(input *CreateCompositeStateInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("invalid input: nil request or task config")
	}
	if input.TaskConfig.Type != task.TaskTypeComposite {
		return fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	return nil
}

func (a *CreateCompositeState) Run(ctx context.Context, input *CreateCompositeStateInput) (*task.State, error) {
	if err := validateCompositeStateInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadCompositeContext(ctx, input)
	if err != nil {
		return nil, err
	}
	normalizedConfig, childConfigs, err := a.normalizeCompositeConfig(
		ctx,
		workflowState,
		workflowConfig,
		input.TaskConfig,
	)
	if err != nil {
		return nil, err
	}
	return a.createCompositeState(ctx, input, workflowState, workflowConfig, normalizedConfig, childConfigs)
}

func (a *CreateCompositeState) loadCompositeContext(
	ctx context.Context,
	input *CreateCompositeStateInput,
) (*workflow.State, *workflow.Config, error) {
	return a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
}

func (a *CreateCompositeState) normalizeCompositeConfig(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.Config, []*task.Config, error) {
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, task.TaskTypeComposite)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create composite normalizer: %w", err)
	}
	variableBuilder := shared.NewVariableBuilder()
	vars := variableBuilder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)
	variableBuilder.AddCurrentInputToVariables(vars, taskConfig.With)
	normContext := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		CurrentInput:   taskConfig.With,
		Variables:      vars,
	}
	normalizedConfig := taskConfig
	if err := normalizer.Normalize(ctx, normalizedConfig, normContext); err != nil {
		return nil, nil, fmt.Errorf("failed to normalize composite task: %w", err)
	}
	childConfigs := make([]*task.Config, len(normalizedConfig.Tasks))
	for i := range normalizedConfig.Tasks {
		childConfigs[i] = &normalizedConfig.Tasks[i]
	}
	return normalizedConfig, childConfigs, nil
}

// createCompositeState manages transactionally creating the composite parent and its artifacts.
func (a *CreateCompositeState) createCompositeState(
	ctx context.Context,
	input *CreateCompositeStateInput,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	normalizedConfig *task.Config,
	childConfigs []*task.Config,
) (*task.State, error) {
	var createdState *task.State
	if err := a.taskRepo.WithTransaction(ctx, func(repo task.Repository) error {
		state, err := a.createCompositeParentState(ctx, repo, workflowState, workflowConfig, input.TaskConfig)
		if err != nil {
			return err
		}
		a.addCompositeMetadata(state, normalizedConfig, len(childConfigs))
		if err := repo.UpsertState(ctx, state); err != nil {
			return fmt.Errorf("failed to update state with composite metadata: %w", err)
		}
		if err := a.storeCompositeArtifacts(ctx, state, normalizedConfig, childConfigs); err != nil {
			return err
		}
		createChildTasksUC := uc.NewCreateChildTasksUC(repo, a.configStore, a.task2Factory, a.cwd)
		if err := createChildTasksUC.Execute(ctx, &uc.CreateChildTasksInput{
			ParentStateID:  state.TaskExecID,
			WorkflowExecID: input.WorkflowExecID,
			WorkflowID:     input.WorkflowID,
		}); err != nil {
			return fmt.Errorf("failed to create child tasks: %w", err)
		}
		createdState = state
		return nil
	}); err != nil {
		return nil, err
	}
	return createdState, nil
}

func (a *CreateCompositeState) storeCompositeArtifacts(
	ctx context.Context,
	state *task.State,
	config *task.Config,
	childConfigs []*task.Config,
) error {
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.cwd)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}
	metadata := &task2core.CompositeTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  childConfigs,
		Strategy:      string(config.GetStrategy()),
		MaxWorkers:    compositeMaxWorkersSingle,
	}
	if err := configRepo.StoreCompositeMetadata(ctx, state.TaskExecID, metadata); err != nil {
		return fmt.Errorf("failed to store composite metadata: %w", err)
	}
	// Store full config so waitForPriorSiblings can determine sibling ordering
	cfgCopy, err := core.DeepCopy(config)
	if err != nil {
		return fmt.Errorf("failed to clone composite config: %w", err)
	}
	if err := a.configStore.Save(ctx, state.TaskExecID.String(), cfgCopy); err != nil {
		return fmt.Errorf("failed to store composite config: %w", err)
	}
	return nil
}

// createCompositeParentState builds the composite parent state inside the transaction boundary.
func (a *CreateCompositeState) createCompositeParentState(
	ctx context.Context,
	repo task.Repository,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.State, error) {
	createStateUC := uc.NewCreateState(repo, a.configStore)
	return createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	})
}

func (a *CreateCompositeState) addCompositeMetadata(state *task.State, config *task.Config, childCount int) {
	if state.Output == nil {
		output := make(core.Output)
		state.Output = &output
	}
	(*state.Output)["composite_metadata"] = map[string]any{
		"child_count": childCount,
		"strategy":    string(config.GetStrategy()),
		"sequential":  true, // Composite tasks are always sequential
	}
}
