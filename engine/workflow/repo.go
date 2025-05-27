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
	LoadExecution(ctx context.Context, workflowExecID core.ID) (*Execution, error)
	LoadExecutionMap(ctx context.Context, workflowExecID core.ID) (map[core.ID]any, error)
	ListExecutions(ctx context.Context) ([]Execution, error)
	ListExecutionsMap(ctx context.Context) ([]map[core.ID]any, error)
	ListExecutionsMapByWorkflowID(ctx context.Context, workflowID core.ID) ([]map[core.ID]any, error)
}
