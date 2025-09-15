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

type CreateCompositeStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateCompositeState handles composite state creation with task2 integration
type CreateCompositeState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
	task2Factory       task2.Factory
	configStore        services.ConfigStore
	cwd                *core.PathCWD
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
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configStore, task2Factory, cwd),
		task2Factory:       task2Factory,
		configStore:        configStore,
		cwd:                cwd,
	}, nil
}

func (a *CreateCompositeState) Run(ctx context.Context, input *CreateCompositeStateInput) (*task.State, error) {
	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeComposite {
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
	// Create parent state first with the original composite config
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 normalizer to prepare composite task configs
	normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeComposite)
	if err != nil {
		return nil, fmt.Errorf("failed to create composite normalizer: %w", err)
	}
	// Build normalization context for composite task
	// This ensures workflow context is available to nested tasks
	variableBuilder := shared.NewVariableBuilder()
	vars := variableBuilder.BuildBaseVariables(workflowState, workflowConfig, input.TaskConfig)
	variableBuilder.AddCurrentInputToVariables(vars, input.TaskConfig.With)

	normContext := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
		CurrentInput:   input.TaskConfig.With,
		Variables:      vars,
	}
	// Normalize the composite task configuration
	normalizedConfig := input.TaskConfig
	if err := normalizer.Normalize(normalizedConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize composite task: %w", err)
	}
	// Get child configs from normalized composite config
	childConfigs := make([]*task.Config, len(normalizedConfig.Tasks))
	for i := range normalizedConfig.Tasks {
		childConfigs[i] = &normalizedConfig.Tasks[i]
	}
	// Store composite metadata using task2 repository
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore, a.cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config repository: %w", err)
	}
	compositeMetadata := &task2core.CompositeTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  childConfigs,
		Strategy:      string(normalizedConfig.GetStrategy()),
		MaxWorkers:    1, // Composite tasks are sequential
	}
	if err := configRepo.StoreCompositeMetadata(ctx, state.TaskExecID, compositeMetadata); err != nil {
		return nil, fmt.Errorf("failed to store composite metadata: %w", err)
	}
	// CRITICAL FIX: Also store the full config so waitForPriorSiblings can find it
	// This enables sequential execution in collection subtasks by allowing
	// waitForPriorSiblings to load the parent config and determine sibling order
	cfgCopy, err := core.DeepCopy(normalizedConfig) // returns *task.Config
	if err != nil {
		return nil, fmt.Errorf("failed to clone composite config: %w", err)
	}
	if err := a.configStore.Save(ctx, state.TaskExecID.String(), cfgCopy); err != nil {
		return nil, fmt.Errorf("failed to store composite config: %w", err)
	}
	// Add metadata to state output
	a.addCompositeMetadata(state, normalizedConfig, len(childConfigs))
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
