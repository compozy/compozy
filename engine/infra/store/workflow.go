package store

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
)

type WorkflowRepository struct {
	store         *Store
	projectConfig *project.Config
	workflows     []*workflow.Config
}

func NewWorkflowRepository(
	store *Store,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *WorkflowRepository {
	return &WorkflowRepository{
		store:         store,
		projectConfig: projectConfig,
		workflows:     workflows,
	}
}

func (r *WorkflowRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.WorkflowMetadata,
	config *workflow.Config,
	input *core.Input,
) (*workflow.Execution, error) {
	projectEnv := r.projectConfig.GetEnv()
	workflowEnv := config.GetEnv()
	requestData, err := workflow.NewRequestData(
		metadata,
		input,
		input,
		projectEnv,
		workflowEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow request data: %w", err)
	}
	execution, err := workflow.NewExecution(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save workflow execution: %w", err)
	}
	return execution, nil
}

func (r *WorkflowRepository) FindConfig(
	workflows []*workflow.Config,
	workflowID string,
) (*workflow.Config, error) {
	config, err := workflow.FindConfig(workflows, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}
	return config, nil
}

func (r *WorkflowRepository) LoadExecution(ctx context.Context, wExecID core.ID) (*workflow.Execution, error) {
	key := workflow.NewStoreKey(wExecID)
	execution, err := GetAndUnmarshalJSON[workflow.Execution](ctx, r.store, key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	return execution, nil
}

func GetExecutionsByComponent[T core.Execution](
	ctx context.Context,
	st *Store,
	wExecID core.ID,
	component core.ComponentType,
) ([]T, error) {
	if component == "" {
		return nil, fmt.Errorf("component cannot be empty")
	}
	prefix := fmt.Appendf(nil, "%s:%s:", string(wExecID), component)
	return GetExecutionsByFilter[T](ctx, st.GetDB(), prefix, nil)
}

func (r *WorkflowRepository) mapFromExecution(
	ctx context.Context,
	execution *workflow.Execution,
) (*core.ExecutionMap, error) {
	taskRepo := NewTaskRepository(r.store, r)
	agentRepo := NewAgentRepository(r.store, r, taskRepo)
	toolRepo := NewToolRepository(r.store, r, taskRepo)
	wExecID := execution.GetID()
	tasksMap, err := taskRepo.LoadExecutionsJSON(ctx, wExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	agentsMap, err := agentRepo.LoadExecutionsJSON(ctx, wExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent executions: %w", err)
	}
	toolsMap, err := toolRepo.LoadExecutionsJSON(ctx, wExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool executions: %w", err)
	}
	return core.NewExecutionMap(execution, tasksMap, agentsMap, toolsMap), nil
}

func (r *WorkflowRepository) LoadExecutionMap(ctx context.Context, wExecID core.ID) (*core.ExecutionMap, error) {
	execution, err := r.LoadExecution(ctx, wExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	return r.mapFromExecution(ctx, execution)
}

func (r *WorkflowRepository) ListExecutions(ctx context.Context) ([]workflow.Execution, error) {
	prefix := []byte("workflow:")
	return GetExecutionsByFilter[workflow.Execution](ctx, r.store.GetDB(), prefix, nil)
}

// TODO: support pagination
func (r *WorkflowRepository) ListExecutionsMap(ctx context.Context) ([]core.ExecutionMap, error) {
	executions, err := r.ListExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	executionMaps := make([]core.ExecutionMap, len(executions))
	for i, execution := range executions {
		executionMap, err := r.mapFromExecution(ctx, &execution)
		if err != nil {
			return nil, fmt.Errorf("failed to map workflow execution: %w", err)
		}
		executionMaps[i] = *executionMap
	}
	return executionMaps, nil
}
