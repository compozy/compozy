package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/pb"
)

type TaskRepository struct {
	store        *Store
	workflowRepo *WorkflowRepository
}

func (s *Store) NewTaskRepository(workflowRepo *WorkflowRepository) *TaskRepository {
	return &TaskRepository{store: s, workflowRepo: workflowRepo}
}

func (r *TaskRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.TaskMetadata,
	config *task.Config,
) (*task.Execution, error) {
	workflowExecID := core.ID(metadata.WorkflowExecId)
	workflowExecution, err := r.workflowRepo.LoadExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}
	workflowEnv := workflowExecution.GetEnv()
	taskEnv := config.GetEnv()
	taskInput := config.GetInput()
	parentInput := workflowExecution.GetInput()
	requestData, err := task.NewRequestData(
		metadata,
		parentInput,
		taskInput,
		workflowEnv,
		taskEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task request data: %w", err)
	}
	execution, err := task.NewExecution(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to create task execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save task execution: %w", err)
	}
	return execution, nil
}

func (r *TaskRepository) LoadExecution(
	ctx context.Context,
	taskExecID core.ID,
) (*task.Execution, error) {
	data, err := r.store.GetTaskExecutionByExecID(ctx, taskExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task execution: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("task execution not found")
	}
	return unmarshalExecution[*task.Execution](*data)
}

func (r *TaskRepository) LoadExecutionsMapByWorkflowExecID(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]any, error) {
	workflowExecID := wExecID.String()
	data, err := r.store.ListTaskExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	executions, err := unmarshalExecutions[*task.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal task executions: %w", err)
	}
	jsonMap := make(map[core.ID]any)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = execution.AsMap()
	}
	return jsonMap, nil
}

// ListExecutionsByWorkflowAndTask lists task executions for a specific workflow and task
func (r *TaskRepository) ListExecutionsByWorkflowAndTask(
	ctx context.Context,
	workflowID, taskID string,
) ([]*task.Execution, error) {
	data, err := r.store.ListTaskExecutionsByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}

	executions, err := unmarshalExecutions[*task.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal task executions: %w", err)
	}

	// Filter by workflow ID
	var filteredExecutions []*task.Execution
	for _, execution := range executions {
		if execution.GetWorkflowID() == workflowID {
			filteredExecutions = append(filteredExecutions, execution)
		}
	}

	return filteredExecutions, nil
}

// ListExecutionsByWorkflow lists all task executions for a specific workflow
func (r *TaskRepository) ListExecutionsByWorkflow(
	ctx context.Context,
	workflowID string,
) ([]*task.Execution, error) {
	data, err := r.store.ListTaskExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}

	executions, err := unmarshalExecutions[*task.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal task executions: %w", err)
	}

	return executions, nil
}

func (r *TaskRepository) ListExecutionsByWorkflowExecID(
	ctx context.Context,
	workflowExecID core.ID,
) ([]*task.Execution, error) {
	data, err := r.store.ListTaskExecutionsByWorkflowExecID(ctx, workflowExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	executions, err := unmarshalExecutions[*task.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal task executions: %w", err)
	}
	return executions, nil
}
