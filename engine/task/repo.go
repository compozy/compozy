package task

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type Repository interface {
	CreateExecution(
		ctx context.Context,
		metadata *pb.TaskMetadata,
		config *Config,
	) (*Execution, error)
	GetExecution(ctx context.Context, taskExecID core.ID) (*Execution, error)
	ListExecutions(ctx context.Context) ([]Execution, error)
	ListExecutionsByTaskID(ctx context.Context, taskID string) ([]Execution, error)
	ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]Execution, error)
	ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]Execution, error)
	ListExecutionsByWorkflowExecID(ctx context.Context, workflowExecID core.ID) ([]Execution, error)
	ListExecutionsByWorkflowAndTaskID(ctx context.Context, workflowID, taskID string) ([]Execution, error)
	ListChildrenExecutions(ctx context.Context, taskExecID core.ID) ([]core.Execution, error)
	ListChildrenExecutionsByTaskID(ctx context.Context, taskID string) ([]core.Execution, error)
	ExecutionsToMap(ctx context.Context, execs []core.Execution) ([]*core.ExecutionMap, error)
}
