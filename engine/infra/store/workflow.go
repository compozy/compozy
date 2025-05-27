package store

import (
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	db "github.com/compozy/compozy/engine/infra/store/sqlc"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
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

func (r *WorkflowRepository) GetExecution(
	ctx context.Context,
	workflowExecID core.ID,
) (*workflow.Execution, error) {
	data, err := r.store.queries.GetWorkflowExecutionByExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow execution: %w", err)
	}
	return core.UnmarshalExecution[*workflow.Execution](data.Data)
}

func (r *WorkflowRepository) ListExecutions(ctx context.Context) ([]workflow.Execution, error) {
	data, err := r.store.queries.ListWorkflowExecutions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions: %w", err)
	}
	return UnmarshalExecutions[workflow.Execution](data)
}

func (r *WorkflowRepository) ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]workflow.Execution, error) {
	execs, err := r.store.queries.ListWorkflowExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions by workflow ID: %w", err)
	}
	return UnmarshalExecutions[workflow.Execution](execs)
}

func (r *WorkflowRepository) ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]workflow.Execution, error) {
	execs, err := r.store.queries.ListWorkflowExecutionsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow executions by status: %w", err)
	}
	return UnmarshalExecutions[workflow.Execution](execs)
}

func (r *WorkflowRepository) ListChildrenExecutions(ctx context.Context, workflowExecID core.ID) ([]core.Execution, error) {
	execs, err := r.store.queries.ListWorkflowChildrenExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow children executions: %w", err)
	}
	tasks, agents, tools, err := r.BuildExecutions(ctx, execs)
	if err != nil {
		return nil, fmt.Errorf("failed to build executions map: %w", err)
	}
	return slices.Concat(tasks, agents, tools), nil
}

func (r *WorkflowRepository) ListChildrenExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]core.Execution, error) {
	execs, err := r.store.queries.ListWorkflowChildrenExecutionsByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow children executions: %w", err)
	}
	tasks, agents, tools, err := r.BuildExecutions(ctx, execs)
	if err != nil {
		return nil, fmt.Errorf("failed to build executions map: %w", err)
	}
	return slices.Concat(tasks, agents, tools), nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (r *WorkflowRepository) ExecutionToMap(
	ctx context.Context,
	execution *workflow.Execution,
) (*core.MainExecutionMap, error) {
	workflowExecID := execution.GetID()
	execs, err := r.store.queries.ListWorkflowChildrenExecutionsByWorkflowExecID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow children executions: %w", err)
	}
	childrenExecs, err := r.BuildExecutionsMap(ctx, execs)
	if err != nil {
		return nil, fmt.Errorf("failed to list children executions: %w", err)
	}
	execMap := execution.AsMainExecMap()
	execMap.WithTasks(childrenExecs.Tasks)
	execMap.WithAgents(childrenExecs.Agents)
	execMap.WithTools(childrenExecs.Tools)
	return execMap, nil
}

func (r *WorkflowRepository) ExecutionsToMap(
	ctx context.Context,
	executions []workflow.Execution,
) ([]*core.MainExecutionMap, error) {
	executionMaps := make([]*core.MainExecutionMap, len(executions))
	for i, execution := range executions {
		execMap, err := r.ExecutionToMap(ctx, &execution)
		if err != nil {
			return nil, fmt.Errorf("failed to convert workflow execution to map: %w", err)
		}
		executionMaps[i] = execMap
	}
	return executionMaps, nil
}

func (r *WorkflowRepository) BuildExecutionsMap(ctx context.Context, execs []db.Execution) (*core.ChildrenExecutionMap, error) {
	taskRepo := r.store.NewTaskRepository(r)
	agentRepo := r.store.NewAgentRepository(r, taskRepo)
	toolRepo := r.store.NewToolRepository(r, taskRepo)
	tasks, agents, tools, err := r.BuildExecutions(ctx, execs)
	tasksMap, err := taskRepo.ExecutionsToMap(ctx, tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tasks to map: %w", err)
	}
	agentsMap, err := agentRepo.ExecutionsToMap(ctx, agents)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agents to map: %w", err)
	}
	toolsMap, err := toolRepo.ExecutionsToMap(ctx, tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tools to map: %w", err)
	}
	return &core.ChildrenExecutionMap{
		Tasks:  tasksMap,
		Agents: agentsMap,
		Tools:  toolsMap,
	}, nil
}

func (r *WorkflowRepository) BuildExecutions(ctx context.Context, execs []db.Execution) (
	[]core.Execution,
	[]core.Execution,
	[]core.Execution,
	error,
) {
	tasks := make([]core.Execution, 0)
	agents := make([]core.Execution, 0)
	tools := make([]core.Execution, 0)
	for _, exec := range execs {
		switch exec.ComponentType {
		case core.ComponentTask:
			item, err := core.UnmarshalExecution[*task.Execution](exec.Data)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to unmarshal task execution: %w", err)
			}
			tasks = append(tasks, item)
		case core.ComponentAgent:
			item, err := core.UnmarshalExecution[*agent.Execution](exec.Data)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to unmarshal agent execution: %w", err)
			}
			agents = append(agents, item)
		case core.ComponentTool:
			item, err := core.UnmarshalExecution[*tool.Execution](exec.Data)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to unmarshal tool execution: %w", err)
			}
			tools = append(tools, item)
		}
	}
	return tasks, agents, tools, nil
}
