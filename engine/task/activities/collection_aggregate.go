package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
)

const AggregateCollectionLabel = "AggregateCollection"

type AggregateCollectionInput struct {
	ParentTaskExecID core.ID                       `json:"parent_task_exec_id"` // Collection task ID
	ItemResults      []ExecuteCollectionItemResult `json:"item_results"`        // Individual results
	TaskConfig       *task.Config                  `json:"task_config"`
}

type AggregateCollection struct {
	handleResponseUC *uc.HandleResponse
	taskRepo         task.Repository
	loadWorkflowUC   *uc.LoadWorkflow
}

// NewAggregateCollection creates a new AggregateCollection activity
func NewAggregateCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *AggregateCollection {
	return &AggregateCollection{
		handleResponseUC: uc.NewHandleResponse(workflowRepo, taskRepo),
		taskRepo:         taskRepo,
		loadWorkflowUC:   uc.NewLoadWorkflow(workflows, workflowRepo),
	}
}

func (a *AggregateCollection) Run(ctx context.Context, input *AggregateCollectionInput) (*task.Response, error) {
	// Get the collection task state
	collectionState, err := a.taskRepo.GetState(ctx, input.ParentTaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection state: %w", err)
	}

	if !collectionState.IsCollection() {
		return nil, fmt.Errorf("task is not a collection task")
	}

	// Update collection state with item results
	err = a.updateCollectionState(collectionState, input.ItemResults)
	if err != nil {
		return nil, fmt.Errorf("failed to update collection state: %w", err)
	}

	// Create collection output
	collectionOutput := a.createCollectionOutput(collectionState, input.ItemResults)

	// Update collection state with final output
	collectionState.Output = collectionOutput

	// Determine final status based on results and strategy
	finalStatus := a.determineFinalStatus(collectionState, input.ItemResults)
	collectionState.UpdateStatus(finalStatus)

	// Persist updated collection state
	if err := a.taskRepo.UpsertState(ctx, collectionState); err != nil {
		return nil, fmt.Errorf("failed to update collection state: %w", err)
	}

	// Handle final response with transitions
	workflowConfig, err := a.getWorkflowConfig(collectionState)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow config: %w", err)
	}

	var executionError error
	if finalStatus == core.StatusFailed {
		executionError = fmt.Errorf("collection execution failed")
	}

	response, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      collectionState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}

	return response, nil
}

func (a *AggregateCollection) updateCollectionState(
	collectionState *task.State,
	itemResults []ExecuteCollectionItemResult,
) error {
	if collectionState.CollectionState == nil {
		return fmt.Errorf("collection state is nil")
	}

	// Reset counters
	collectionState.ProcessedCount = 0
	collectionState.CompletedCount = 0
	collectionState.FailedCount = 0

	// Update item results and counters
	for _, result := range itemResults {
		collectionState.ProcessedCount++

		// Ensure ItemResults slice is large enough
		if len(collectionState.ItemResults) <= result.ItemIndex {
			newResults := make([]string, result.ItemIndex+1)
			copy(newResults, collectionState.ItemResults)
			collectionState.ItemResults = newResults
		}

		// Store the task execution ID
		collectionState.ItemResults[result.ItemIndex] = result.TaskExecID.String()

		// Update status counters
		switch result.Status {
		case core.StatusSuccess:
			collectionState.CompletedCount++
		case core.StatusFailed:
			collectionState.FailedCount++
		}
	}

	return nil
}

func (a *AggregateCollection) createCollectionOutput(
	collectionState *task.State,
	itemResults []ExecuteCollectionItemResult,
) *core.Output {
	// Create results array with individual item outputs
	results := make([]map[string]any, len(itemResults))
	for i, result := range itemResults {
		resultItem := map[string]any{
			"index":  result.ItemIndex,
			"status": result.Status,
		}

		// Add item data if available
		if i < len(collectionState.Items) {
			resultItem["item"] = collectionState.Items[result.ItemIndex]
		}

		// Add output if successful
		if result.Output != nil {
			resultItem["output"] = result.Output
		}

		// Add error if failed
		if result.Error != nil {
			resultItem["error"] = result.Error
		}

		results[i] = resultItem
	}

	// Create summary
	summary := map[string]any{
		"total_items":    len(collectionState.Items),
		"filtered_items": len(itemResults),
		"completed":      collectionState.CompletedCount,
		"failed":         collectionState.FailedCount,
		"skipped":        len(collectionState.Items) - len(itemResults),
		"mode":           collectionState.Mode,
	}

	output := &core.Output{
		"results": results,
		"summary": summary,
	}

	return output
}

func (a *AggregateCollection) determineFinalStatus(
	collectionState *task.State,
	itemResults []ExecuteCollectionItemResult,
) core.StatusType {
	if collectionState.CollectionState == nil {
		return core.StatusFailed
	}

	collectionConfig := collectionState.CollectionState
	failedCount := collectionConfig.FailedCount
	totalCount := len(itemResults)

	// If continue_on_error is true, only fail if ALL items failed
	if collectionConfig.ContinueOnError {
		if failedCount == totalCount && totalCount > 0 {
			return core.StatusFailed
		}
		return core.StatusSuccess
	}

	// If continue_on_error is false, fail if ANY item failed
	if failedCount > 0 {
		return core.StatusFailed
	}

	return core.StatusSuccess
}

func (a *AggregateCollection) getWorkflowConfig(collectionState *task.State) (*workflow.Config, error) {
	// Load workflow config using existing infrastructure
	_, workflowConfig, err := a.loadWorkflowUC.Execute(context.Background(), &uc.LoadWorkflowInput{
		WorkflowID:     collectionState.WorkflowID,
		WorkflowExecID: collectionState.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow config: %w", err)
	}
	return workflowConfig, nil
}
