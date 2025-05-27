package workflow

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type Repository interface {
	FindConfig(workflows []*Config, workflowID string) (*Config, error)
	CreateExecution(
		ctx context.Context,
		metadata *pb.WorkflowMetadata,
		config *Config,
		input *core.Input,
	) (*Execution, error)
	GetExecution(ctx context.Context, workflowExecID core.ID) (*Execution, error)
	ListExecutions(ctx context.Context) ([]Execution, error)
	ListExecutionsByStatus(ctx context.Context, status core.StatusType) ([]Execution, error)
	ListExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]Execution, error)
	ListChildrenExecutions(ctx context.Context, workflowExecID core.ID) ([]core.Execution, error)
	ListChildrenExecutionsByWorkflowID(ctx context.Context, workflowID string) ([]core.Execution, error)
	ExecutionToMap(ctx context.Context, execution *Execution) (*core.MainExecutionMap, error)
	ExecutionsToMap(ctx context.Context, executions []Execution) ([]*core.MainExecutionMap, error)
}
