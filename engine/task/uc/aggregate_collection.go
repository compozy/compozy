package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

type AggregateCollectionInput struct {
	CollectionState *task.State
	ItemResults     []CollectionItemResult
}

type CollectionItemResult struct {
	ItemIndex  int             `json:"item_index"`
	TaskExecID core.ID         `json:"task_exec_id"`
	Status     core.StatusType `json:"status"`
	Output     *core.Output    `json:"output,omitempty"`
	Error      *core.Error     `json:"error,omitempty"`
}

type AggregateCollectionResult struct {
	FinalStatus core.StatusType
	Output      *core.Output
}

type AggregateCollection struct {
	taskRepo task.Repository
}

func NewAggregateCollection(taskRepo task.Repository) *AggregateCollection {
	return &AggregateCollection{
		taskRepo: taskRepo,
	}
}

func (uc *AggregateCollection) Execute(ctx context.Context, input *AggregateCollectionInput) (*AggregateCollectionResult, error) {
	collectionState := input.CollectionState
	itemResults := input.ItemResults

	// Update collection state with item results
	if err := uc.updateCollectionState(collectionState, itemResults); err != nil {
		return nil, fmt.Errorf("failed to update collection state: %w", err)
	}

	// Create and set collection output
	collectionOutput := uc.createCollectionOutput(collectionState, itemResults)
	collectionState.Output = collectionOutput

	// Determine final status
	finalStatus := uc.determineFinalStatus(collectionState, itemResults)
	collectionState.UpdateStatus(finalStatus)

	// Persist updated collection state
	if err := uc.taskRepo.UpsertState(ctx, collectionState); err != nil {
		return nil, fmt.Errorf("failed to update collection state: %w", err)
	}

	return &AggregateCollectionResult{
		FinalStatus: finalStatus,
		Output:      collectionOutput,
	}, nil
}

func (uc *AggregateCollection) updateCollectionState(
	collectionState *task.State,
	itemResults []CollectionItemResult,
) error {
	if collectionState.CollectionState == nil {
		return fmt.Errorf("collection state is nil")
	}

	// Reset counters
	collectionState.CollectionState.ProcessedCount = 0
	collectionState.CollectionState.CompletedCount = 0
	collectionState.CollectionState.FailedCount = 0

	// Update item results and counters
	for _, result := range itemResults {
		collectionState.CollectionState.ProcessedCount++

		// Ensure ItemResults slice is large enough
		if len(collectionState.CollectionState.ItemResults) <= result.ItemIndex {
			newResults := make([]string, result.ItemIndex+1)
			copy(newResults, collectionState.CollectionState.ItemResults)
			collectionState.CollectionState.ItemResults = newResults
		}

		// Store the task execution ID
		collectionState.CollectionState.ItemResults[result.ItemIndex] = result.TaskExecID.String()

		// Update status counters
		switch result.Status {
		case core.StatusSuccess:
			collectionState.CollectionState.CompletedCount++
		case core.StatusFailed:
			collectionState.CollectionState.FailedCount++
		}
	}

	return nil
}

func (uc *AggregateCollection) createCollectionOutput(
	collectionState *task.State,
	itemResults []CollectionItemResult,
) *core.Output {
	// Create results array with individual item outputs
	results := make([]map[string]any, len(itemResults))
	for i, result := range itemResults {
		resultItem := map[string]any{
			"index":  result.ItemIndex,
			"status": result.Status,
		}

		// Add item data if available
		if i < len(collectionState.CollectionState.Items) {
			resultItem["item"] = collectionState.CollectionState.Items[result.ItemIndex]
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
		"total_items":    len(collectionState.CollectionState.Items),
		"filtered_items": len(itemResults),
		"completed":      collectionState.CollectionState.CompletedCount,
		"failed":         collectionState.CollectionState.FailedCount,
		"skipped":        len(collectionState.CollectionState.Items) - len(itemResults),
		"mode":           collectionState.CollectionState.Mode,
	}

	output := &core.Output{
		"results": results,
		"summary": summary,
	}

	return output
}

func (uc *AggregateCollection) determineFinalStatus(
	collectionState *task.State,
	itemResults []CollectionItemResult,
) core.StatusType {
	if collectionState.CollectionState == nil {
		return core.StatusFailed
	}

	collectionConfig := collectionState.CollectionState
	failedCount := collectionConfig.FailedCount
	totalCount := len(itemResults)

	// Handle edge case: no items to process
	if totalCount == 0 {
		return core.StatusSuccess // Empty collection is considered successful
	}

	// If continue_on_error is true, only fail if ALL items failed
	if collectionConfig.ContinueOnError {
		if failedCount == totalCount {
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
