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

func (s *Store) NewWorkflowRepository(
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *WorkflowRepository {
	return &WorkflowRepository{
		store:         s,
		projectConfig: projectConfig,
		workflows:     workflows,
	}
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

func (r *WorkflowRepository) LoadExecution(
	ctx context.Context,
	workflowExecID core.ID,
) (*workflow.Execution, error) {
	data, err := r.store.GetWorkflowExecutionByExecID(ctx, workflowExecID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("workflow execution not found")
	}
	return unmarshalExecution[*workflow.Execution](*data)
}

func (r *WorkflowRepository) LoadExecutionMap(
	ctx context.Context,
	workflowExecID core.ID,
) (map[core.ID]any, error) {
	execution, err := r.LoadExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	return r.mapFromExecution(ctx, execution)
}

func (r *WorkflowRepository) ListExecutions(ctx context.Context) ([]workflow.Execution, error) {
	data, err := r.store.ListWorkflowExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	return unmarshalExecutions[workflow.Execution](data)
}

// TODO: support pagination
func (r *WorkflowRepository) ListExecutionsMap(ctx context.Context) ([]map[core.ID]any, error) {
	executions, err := r.ListExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	executionMaps := make([]map[core.ID]any, len(executions))
	for i, execution := range executions {
		executionMap, err := r.mapFromExecution(ctx, &execution)
		if err != nil {
			return nil, fmt.Errorf("failed to map workflow execution: %w", err)
		}
		executionMaps[i] = executionMap
	}
	return executionMaps, nil
}

// TODO: support pagination
func (r *WorkflowRepository) ListExecutionsMapByWorkflowID(
	ctx context.Context,
	workflowID core.ID,
) ([]map[core.ID]any, error) {
	data, err := r.store.ListWorkflowExecutionsByWorkflowID(ctx, workflowID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	executions, err := unmarshalExecutions[workflow.Execution](data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow executions: %w", err)
	}
	executionMaps := make([]map[core.ID]any, len(executions))
	for i, execution := range executions {
		executionMap, err := r.mapFromExecution(ctx, &execution)
		if err != nil {
			return nil, fmt.Errorf("failed to map workflow execution: %w", err)
		}
		executionMaps[i] = executionMap
	}
	return executionMaps, nil
}

func (r *WorkflowRepository) mapFromExecution(
	ctx context.Context,
	execution *workflow.Execution,
) (map[core.ID]any, error) {
	workflowExecID := execution.GetID()
	taskRepo := r.store.NewTaskRepository(r)
	agentRepo := r.store.NewAgentRepository(r, taskRepo)
	toolRepo := r.store.NewToolRepository(r, taskRepo)
	tasksMap, err := taskRepo.LoadExecutionsMapByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions: %w", err)
	}
	agentsMap, err := agentRepo.LoadExecutionsMapByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent executions: %w", err)
	}
	toolsMap, err := toolRepo.LoadExecutionsMapByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool executions: %w", err)
	}
	execMap := execution.AsMap()
	execMap["tasks"] = tasksMap
	execMap["agents"] = agentsMap
	execMap["tools"] = toolsMap
	return execMap, nil
}
