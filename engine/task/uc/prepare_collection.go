package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/normalizer"
)

type PrepareCollectionInput struct {
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
}

type PrepareCollectionResult struct {
	TaskExecID      core.ID     `json:"task_exec_id"`
	FilteredCount   int         `json:"filtered_count"`
	TotalCount      int         `json:"total_count"`
	BatchCount      int         `json:"batch_count"`
	CollectionState *task.State `json:"collection_state"`
}

type PrepareCollection struct {
	createStateUC       *CreateState
	taskRepo            task.Repository
	collectionEvaluator *CollectionEvaluator
}

func NewPrepareCollection(taskRepo task.Repository) *PrepareCollection {
	return &PrepareCollection{
		createStateUC:       NewCreateState(taskRepo),
		taskRepo:            taskRepo,
		collectionEvaluator: NewCollectionEvaluator(),
	}
}

func (uc *PrepareCollection) Execute(ctx context.Context, input *PrepareCollectionInput) (*PrepareCollectionResult, error) {
	// Evaluate collection items
	result, err := uc.evaluateCollectionItems(input.TaskConfig, input.WorkflowState, input.WorkflowConfig)
	if err != nil {
		return nil, err
	}

	// Calculate batch information
	batchCount := uc.calculateBatchCount(result.FilteredCount, input.TaskConfig.GetBatch(), input.TaskConfig.GetMode())

	// Create and persist collection state
	collectionState, err := uc.createCollectionState(ctx, input.WorkflowState, input.WorkflowConfig, input.TaskConfig, result.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection state: %w", err)
	}

	return &PrepareCollectionResult{
		TaskExecID:      collectionState.TaskExecID,
		FilteredCount:   result.FilteredCount,
		TotalCount:      result.TotalCount,
		BatchCount:      batchCount,
		CollectionState: collectionState,
	}, nil
}

func (uc *PrepareCollection) evaluateCollectionItems(
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*EvaluationResult, error) {
	// Build evaluation context
	evaluationContext := uc.buildEvaluationContext(taskConfig, workflowState, workflowConfig)

	// Evaluate collection items using shared service
	evalInput := &EvaluationInput{
		ItemsExpr:  taskConfig.Items,
		FilterExpr: taskConfig.Filter,
		Context:    evaluationContext,
		ItemVar:    taskConfig.GetItemVar(),
		IndexVar:   taskConfig.GetIndexVar(),
	}

	result, err := uc.collectionEvaluator.EvaluateItems(context.Background(), evalInput)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate collection items: %w", err)
	}

	return result, nil
}

func (uc *PrepareCollection) buildEvaluationContext(
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) map[string]any {
	// Use normalizer to build proper context structure
	contextBuilder := normalizer.NewContextBuilder()
	taskConfigs := normalizer.BuildTaskConfigsMap([]task.Config{*taskConfig})

	// Merge environments: workflow -> task
	envMerger := &core.EnvMerger{}
	mergedEnv, err := envMerger.MergeWithDefaults(
		workflowConfig.GetEnv(),
		taskConfig.GetEnv(),
	)
	if err != nil {
		// If merge fails, use task env only
		mergedEnv = taskConfig.GetEnv()
	}

	normCtx := &normalizer.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		ParentConfig:   nil,
		CurrentInput:   taskConfig.With,
		MergedEnv:      &mergedEnv,
	}

	return contextBuilder.BuildContext(normCtx)
}

func (uc *PrepareCollection) calculateBatchCount(itemCount, batchSize int, mode task.CollectionMode) int {
	if mode == task.CollectionModeSequential && batchSize > 0 {
		return (itemCount + batchSize - 1) / batchSize // Ceiling division
	}
	return 1 // For parallel mode, treat as single batch
}

func (uc *PrepareCollection) createCollectionState(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	filteredItems []any,
) (*task.State, error) {
	collectionTask := &taskConfig.CollectionTask

	// Create collection state with evaluated items
	partialState := task.CreateCollectionPartialState(
		filteredItems,
		collectionTask.Filter,
		string(collectionTask.GetMode()),
		collectionTask.GetBatch(),
		collectionTask.ContinueOnError,
		collectionTask.GetItemVar(),
		collectionTask.GetIndexVar(),
		nil, // ParallelConfig will be set if needed
		taskConfig.Env,
	)

	// Create and persist the collection state
	taskExecID := core.MustNewID()
	stateInput := &task.CreateStateInput{
		WorkflowID:     workflowConfig.ID,
		WorkflowExecID: workflowState.WorkflowExecID,
		TaskID:         taskConfig.ID,
		TaskExecID:     taskExecID,
	}

	collectionState := task.CreateCollectionState(stateInput, partialState)

	// Persist the collection state to the database
	if err := uc.taskRepo.UpsertState(ctx, collectionState); err != nil {
		return nil, fmt.Errorf("failed to persist collection state: %w", err)
	}

	return collectionState, nil
}
