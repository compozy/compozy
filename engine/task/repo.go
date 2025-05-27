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
	LoadExecution(ctx context.Context, taskExecID core.ID) (*Execution, error)
	LoadExecutionsMapByWorkflowExecID(ctx context.Context, wExecID core.ID) (map[core.ID]any, error)
	ListExecutionsByWorkflowAndTask(ctx context.Context, workflowID, taskID string) ([]*Execution, error)
	ListExecutionsByWorkflow(ctx context.Context, workflowID string) ([]*Execution, error)
	ListExecutionsByWorkflowExecID(ctx context.Context, workflowExecID core.ID) ([]*Execution, error)
}
