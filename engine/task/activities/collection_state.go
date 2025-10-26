package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/tasks"
	taskcore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateCollectionStateLabel = "CreateCollectionState"

const (
	outputKeyCollectionMetadata = "collection_metadata"
	outKeyItemCount             = "item_count"
	outKeySkippedCount          = "skipped_count"
	outKeyChildCount            = "child_count"
)

type CreateCollectionStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

// CreateCollectionState handles collection state creation with tasks integration
type CreateCollectionState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	createChildTasksUC *uc.CreateChildTasks
	tasksFactory       tasks.Factory
	configStore        services.ConfigStore
	taskRepo           task.Repository
	cwd                *core.PathCWD
}

// NewCreateCollectionState creates a new CreateCollectionState activity with tasks integration
func NewCreateCollectionState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
	tasksFactory tasks.Factory,
) (*CreateCollectionState, error) {
	return &CreateCollectionState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configStore),
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configStore, tasksFactory, cwd),
		tasksFactory:       tasksFactory,
		configStore:        configStore,
		taskRepo:           taskRepo,
		cwd:                cwd,
	}, nil
}

func (a *CreateCollectionState) Run(ctx context.Context, input *CreateCollectionStateInput) (*task.State, error) {
	if err := validateCollectionStateInput(input); err != nil {
		return nil, err
	}
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}
	expansionResult, err := a.expandCollectionItems(ctx, input.TaskConfig, workflowState, workflowConfig)
	if err != nil {
		return nil, err
	}
	return a.createCollectionState(ctx, input, workflowState, workflowConfig, expansionResult)
}

func (a *CreateCollectionState) addCollectionMetadata(state *task.State, result *shared.ExpansionResult) {
	if state.Output == nil {
		output := make(core.Output)
		state.Output = &output
	}
	(*state.Output)[outputKeyCollectionMetadata] = map[string]any{
		outKeyItemCount:    result.ItemCount,
		outKeySkippedCount: result.SkippedCount,
		outKeyChildCount:   len(result.ChildConfigs),
	}
}

// validateCollectionStateInput ensures the incoming request is usable.
func validateCollectionStateInput(input *CreateCollectionStateInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("invalid input: nil request or task config")
	}
	if input.TaskConfig.Type != task.TaskTypeCollection {
		return fmt.Errorf("unsupported task type: %s", input.TaskConfig.Type)
	}
	return nil
}

// expandCollectionItems runs the tasks expander and validates the resulting metadata.
func (a *CreateCollectionState) expandCollectionItems(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*shared.ExpansionResult, error) {
	expander := a.tasksFactory.CreateCollectionExpander(ctx)
	expansionResult, err := expander.ExpandItems(ctx, taskConfig, workflowState, workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to expand collection items: %w", err)
	}
	if err := expander.ValidateExpansion(ctx, expansionResult); err != nil {
		return nil, fmt.Errorf("expansion validation failed: %w", err)
	}
	return expansionResult, nil
}

// createCollectionState orchestrates transactional state creation and metadata persistence.
func (a *CreateCollectionState) createCollectionState(
	ctx context.Context,
	input *CreateCollectionStateInput,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	expansionResult *shared.ExpansionResult,
) (*task.State, error) {
	var createdState *task.State
	if err := a.taskRepo.WithTransaction(ctx, func(repo task.Repository) error {
		state, err := a.createCollectionParentState(ctx, repo, workflowState, workflowConfig, input.TaskConfig)
		if err != nil {
			return err
		}
		if err := a.storeCollectionArtifacts(ctx, state, input.TaskConfig, expansionResult); err != nil {
			return err
		}
		if err := repo.UpsertState(ctx, state); err != nil {
			return fmt.Errorf("failed to update state with collection metadata: %w", err)
		}
		createChildTasksUC := uc.NewCreateChildTasksUC(repo, a.configStore, a.tasksFactory, a.cwd)
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

// createCollectionParentState builds the parent state within the transaction scope.
func (a *CreateCollectionState) createCollectionParentState(
	ctx context.Context,
	repo task.Repository,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*task.State, error) {
	createStateUC := uc.NewCreateState(repo, a.configStore)
	state, err := createStateUC.Execute(ctx, &uc.CreateStateInput{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

// storeCollectionArtifacts persists metadata and enrichment artifacts for the collection execution.
func (a *CreateCollectionState) storeCollectionArtifacts(
	ctx context.Context,
	state *task.State,
	taskConfig *task.Config,
	expansionResult *shared.ExpansionResult,
) error {
	configRepo, err := a.tasksFactory.CreateTaskConfigRepository(a.configStore, a.cwd)
	if err != nil {
		return fmt.Errorf("failed to create task config repository: %w", err)
	}
	collectionMetadata := &taskcore.CollectionTaskMetadata{
		ParentStateID: state.TaskExecID,
		ChildConfigs:  expansionResult.ChildConfigs,
		Strategy:      string(taskConfig.GetStrategy()),
		MaxWorkers:    taskConfig.GetMaxWorkers(),
		Mode:          string(taskConfig.GetMode()),
		BatchSize:     taskConfig.Batch,
		ItemCount:     expansionResult.ItemCount,
		SkippedCount:  expansionResult.SkippedCount,
	}
	if err := configRepo.StoreCollectionMetadata(ctx, state.TaskExecID, collectionMetadata); err != nil {
		return fmt.Errorf("failed to store collection metadata: %w", err)
	}
	a.addCollectionMetadata(state, expansionResult)
	return nil
}
