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
	loadCollectionStateUC *uc.LoadCollectionState
	aggregateCollectionUC *uc.AggregateCollection
	handleResponseUC      *uc.HandleResponse
	loadWorkflowUC        *uc.LoadWorkflow
}

// NewAggregateCollection creates a new AggregateCollection activity
func NewAggregateCollection(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
) *AggregateCollection {
	return &AggregateCollection{
		loadCollectionStateUC: uc.NewLoadCollectionState(taskRepo),
		aggregateCollectionUC: uc.NewAggregateCollection(taskRepo),
		handleResponseUC:      uc.NewHandleResponse(workflowRepo, taskRepo),
		loadWorkflowUC:        uc.NewLoadWorkflow(workflows, workflowRepo),
	}
}

func (a *AggregateCollection) Run(ctx context.Context, input *AggregateCollectionInput) (*task.Response, error) {
	// Load and validate collection state
	collectionState, err := a.loadCollectionStateUC.Execute(ctx, &uc.LoadCollectionStateInput{
		TaskExecID: input.ParentTaskExecID,
	})
	if err != nil {
		return nil, err
	}

	// Convert ExecuteCollectionItemResult to uc.CollectionItemResult
	itemResults := make([]uc.CollectionItemResult, len(input.ItemResults))
	for i, result := range input.ItemResults {
		itemResults[i] = uc.CollectionItemResult{
			ItemIndex:  result.ItemIndex,
			TaskExecID: result.TaskExecID,
			Status:     result.Status,
			Output:     result.Output,
			Error:      result.Error,
		}
	}

	// Process and finalize collection results
	aggregateResult, err := a.aggregateCollectionUC.Execute(ctx, &uc.AggregateCollectionInput{
		CollectionState: collectionState,
		ItemResults:     itemResults,
	})
	if err != nil {
		return nil, err
	}

	// Generate final response
	return a.generateFinalResponse(ctx, collectionState, input.TaskConfig, aggregateResult.FinalStatus)
}



func (a *AggregateCollection) generateFinalResponse(
	ctx context.Context,
	collectionState *task.State,
	taskConfig *task.Config,
	finalStatus core.StatusType,
) (*task.Response, error) {
	_, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     collectionState.WorkflowID,
		WorkflowExecID: collectionState.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow config: %w", err)
	}

	var executionError error
	if finalStatus == core.StatusFailed {
		executionError = fmt.Errorf("collection execution failed")
	}

	response, err := a.handleResponseUC.Execute(ctx, &uc.HandleResponseInput{
		TaskState:      collectionState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to handle collection response: %w", err)
	}

	return response, nil
}
