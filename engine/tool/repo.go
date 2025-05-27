package tool

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type Repository interface {
	CreateExecution(
		ctx context.Context,
		metadata *pb.ToolMetadata,
		config *Config,
	) (*Execution, error)
	GetExecution(ctx context.Context, toolExecID core.ID) (*Execution, error)
	ListExecutions(ctx context.Context) ([]Execution, error)
	ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]Execution, error)
	ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]Execution, error)
	ListExecutionsByWorkflowExecID(ctx context.Context, workflowExecID core.ID) ([]Execution, error)
	ListExecutionsByTaskID(ctx context.Context, taskID string) ([]Execution, error)
	ListExecutionsByTaskExecID(ctx context.Context, taskExecID core.ID) ([]Execution, error)
	ListExecutionsByToolID(ctx context.Context, toolID string) ([]Execution, error)
	ExecutionsToMap(ctx context.Context, execs []core.Execution) ([]*core.ExecutionMap, error)
}
