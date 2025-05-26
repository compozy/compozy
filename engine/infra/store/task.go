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

func NewTaskRepository(store *Store, workflowRepo *WorkflowRepository) *TaskRepository {
	return &TaskRepository{
		store:        store,
		workflowRepo: workflowRepo,
	}
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
	wExecID core.ID,
	taskExecID core.ID,
) (*task.Execution, error) {
	key := task.NewStoreKey(wExecID, taskExecID)
	execution, err := GetAndUnmarshalJSON[task.Execution](ctx, r.store, key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get task execution: %w", err)
	}
	return execution, nil
}

func (r *TaskRepository) LoadExecutionsJSON(
	ctx context.Context,
	wExecID core.ID,
) (map[core.ID]core.JSONMap, error) {
	executions, err := GetExecutionsByComponent[*task.Execution](ctx, r.store, wExecID, core.ComponentTask)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	jsonMap := make(map[core.ID]core.JSONMap)
	for _, execution := range executions {
		jsonMap[execution.GetID()] = core.JSONMapFromExecution(execution)
	}
	return jsonMap, nil
}
