package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type AgentRepository struct {
	store        *Store
	workflowRepo *WorkflowRepository
	taskRepo     *TaskRepository
}

func (s *Store) NewAgentRepository(
	workflowRepo *WorkflowRepository,
	taskRepo *TaskRepository,
) *AgentRepository {
	return &AgentRepository{
		store:        s,
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
	}
}

func (r *AgentRepository) CreateExecution(
	ctx context.Context,
	metadata *pb.AgentMetadata,
	config *agent.Config,
) (*agent.Execution, error) {
	workflowExecID := core.ID(metadata.WorkflowExecId)
	workflowExecution, err := r.workflowRepo.GetExecution(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow execution: %w", err)
	}
	taskExecID := core.ID(metadata.TaskExecId)
	taskExecution, err := r.taskRepo.GetExecution(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load task execution: %w", err)
	}

	// Get the main execution map for template context
	mainExecMap, err := r.workflowRepo.ExecutionToMap(ctx, workflowExecution)
	if err != nil {
		return nil, fmt.Errorf("failed to get main execution map: %w", err)
	}

	parentInput := workflowExecution.GetInput()
	agentEnv := config.GetEnv()
	taskEnv := taskExecution.GetEnv()
	taskInput := taskExecution.GetInput()
	agentInput := config.GetInput()
	requestData, err := agent.NewRequestData(
		metadata,
		parentInput,
		taskInput,
		agentInput,
		taskEnv,
		agentEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent request data: %w", err)
	}
	execution, err := agent.NewExecutionWithContext(requestData, mainExecMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent execution: %w", err)
	}
	key := execution.StoreKey()
	if err := r.store.UpsertJSON(ctx, key, execution); err != nil {
		return nil, fmt.Errorf("failed to save agent execution: %w", err)
	}
	return execution, nil
}

func (r *AgentRepository) GetExecution(
	ctx context.Context,
	agentExecID core.ID,
) (*agent.Execution, error) {
	data, err := r.store.queries.GetAgentExecutionByExecID(ctx, agentExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent execution: %w", err)
	}
	return core.UnmarshalExecution[*agent.Execution](data.Data)
}

func (r *AgentRepository) ListExecutions(ctx context.Context) ([]agent.Execution, error) {
	execs, err := r.store.queries.ListAgentExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByStatus(
	ctx context.Context,
	status core.StatusType,
) ([]agent.Execution, error) {
	execs, err := r.store.queries.ListAgentExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by status: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByWorkflowID(
	ctx context.Context,
	workflowID string,
) ([]agent.Execution, error) {
	execs, err := r.store.queries.ListAgentExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by workflow ID: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByWorkflowExecID(
	ctx context.Context,
	workflowExecID core.ID,
) ([]agent.Execution, error) {
	execs, err := r.store.queries.ListAgentExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by workflow exec ID: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByTaskID(ctx context.Context, taskID string) ([]agent.Execution, error) {
	execID := sql.NullString{String: taskID, Valid: true}
	execs, err := r.store.queries.ListAgentExecutionsByTaskID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by task ID: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByTaskExecID(
	ctx context.Context,
	taskExecID core.ID,
) ([]agent.Execution, error) {
	execs, err := r.store.queries.ListAgentExecutionsByTaskExecID(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by task exec ID: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

func (r *AgentRepository) ListExecutionsByAgentID(ctx context.Context, agentID string) ([]agent.Execution, error) {
	execID := sql.NullString{String: agentID, Valid: true}
	execs, err := r.store.queries.ListAgentExecutionsByAgentID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent executions by agent ID: %w", err)
	}
	return UnmarshalExecutions[agent.Execution](execs)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (r *AgentRepository) ExecutionsToMap(_ context.Context, execs []core.Execution) ([]*core.ExecutionMap, error) {
	execMaps := make([]*core.ExecutionMap, len(execs))
	for i, exec := range execs {
		execMaps[i] = exec.AsExecMap()
	}
	return execMaps, nil
}
