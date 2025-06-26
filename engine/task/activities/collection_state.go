package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const CreateCollectionStateLabel = "CreateCollectionState"

type CreateCollectionStateInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
}

type CreateCollectionState struct {
	loadWorkflowUC     *uc.LoadWorkflow
	createStateUC      *uc.CreateState
	configManager      *services.ConfigManager
	createChildTasksUC *uc.CreateChildTasks
}

// NewCreateCollectionState creates a new CreateCollectionState activity
func NewCreateCollectionState(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	cwd *core.PathCWD,
) (*CreateCollectionState, error) {
	configManager, err := services.NewConfigManager(configStore, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}
	return &CreateCollectionState{
		loadWorkflowUC:     uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:      uc.NewCreateState(taskRepo, configManager),
		configManager:      configManager,
		createChildTasksUC: uc.NewCreateChildTasksUC(taskRepo, configManager),
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
	collectionMetadata, err := a.configManager.PrepareCollectionConfigs(
		ctx,
		state.TaskExecID,
		input.TaskConfig,
		workflowState,
		workflowConfig,
	)
	if err != nil {
		return nil, err
	}
	a.addCollectionMetadata(state, collectionMetadata)
	if err := a.createChildTasksUC.Execute(ctx, &uc.CreateChildTasksInput{
		ParentStateID:  state.TaskExecID,
		WorkflowExecID: input.WorkflowExecID,
		WorkflowID:     input.WorkflowID,
	}); err != nil {
		return nil, fmt.Errorf("failed to create child tasks: %w", err)
	}

	return state, nil
}

func (a *CreateCollectionState) addCollectionMetadata(state *task.State, metadata *services.CollectionMetadata) {
	if state.Output == nil {
		output := make(core.Output)
		state.Output = &output
	}
	(*state.Output)["collection_metadata"] = map[string]any{
		"item_count":    metadata.ItemCount,
		"skipped_count": metadata.SkippedCount,
		"total_items":   metadata.ItemCount + metadata.SkippedCount,
		"mode":          metadata.Mode,
		"batch_size":    metadata.BatchSize,
	}
}
