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

const CreateCollectionStateLabel = "CreateCollectionState"

type CreateCollectionStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateCollectionState handles collection state creation with task2 integration
type CreateCollectionState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
	task2Factory       task2.Factory
	configStore        services.ConfigStore
	taskRepo           task.Repository
}

// NewCreateCollectionState creates a new CreateCollectionState activity with task2 integration
func NewCreateCollectionState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	_ *core.PathCWD,
	task2Factory task2.Factory,
) (*CreateCollectionState, error) {
	return &CreateCollectionState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configStore, task2Factory),
		task2Factory:       task2Factory,
		configStore:        configStore,
		taskRepo:           taskRepo,
	}, nil
}

func (a *CreateCollectionState) Run(ctx context.Context, input *CreateCollectionStateInput) (*task.State, error) {
	// Validate task type
	if input.TaskConfig.Type != task.TaskTypeCollection {
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
	// Create parent state first with the original collection config
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
	})
	if err != nil {
		return nil, err
	}
	// Use task2 CollectionExpander
	expander := a.task2Factory.CreateCollectionExpander()
	expansionResult, err := expander.ExpandItems(ctx, input.TaskConfig, workflowState, workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to expand collection items: %w", err)
	}
	// Validate expansion result
	if err := expander.ValidateExpansion(expansionResult); err != nil {
		return nil, fmt.Errorf("expansion validation failed: %w", err)
	}
	// Store expanded configs for child task creation
	configRepo, err := a.task2Factory.CreateTaskConfigRepository(a.configStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create task config repository: %w", err)
	}
	collectionMetadata := &task2core.CollectionTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  expansionResult.ChildConfigs,
		ItemCount:     expansionResult.ItemCount,
		SkippedCount:  expansionResult.SkippedCount,
	}
	if err := configRepo.StoreCollectionMetadata(ctx, state.TaskExecID, collectionMetadata); err != nil {
		return nil, fmt.Errorf("failed to store collection metadata: %w", err)
	}
	// Add metadata to state output
	a.addCollectionMetadata(state, expansionResult)
	// Update the state in the repository to persist the metadata
	if err := a.taskRepo.UpsertState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to update state with collection metadata: %w", err)
	}
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

func (a *CreateCollectionState) addCollectionMetadata(state *task.State, result *shared.ExpansionResult) {
	if state.Output == nil {
		output := make(core.Output)
		state.Output = &output
	}
	(*state.Output)["collection_metadata"] = map[string]any{
		"item_count":    result.ItemCount,
		"skipped_count": result.SkippedCount,
		"child_count":   len(result.ChildConfigs),
	}
}
