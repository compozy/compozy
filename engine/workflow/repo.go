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
	LoadExecution(ctx context.Context, wExecID core.ID) (*Execution, error)
	LoadExecutionMap(ctx context.Context, wExecID core.ID) (*core.ExecutionMap, error)
	ListExecutions(ctx context.Context) ([]Execution, error)
	ListExecutionsMap(ctx context.Context) ([]core.ExecutionMap, error)
}
