package agent

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
)

type Repository interface {
	CreateExecution(
		ctx context.Context,
		metadata *pb.AgentMetadata,
		config *Config,
	) (*Execution, error)
	LoadExecution(ctx context.Context, agentExecID core.ID) (*Execution, error)
	LoadExecutionsMapByWorkflowExecID(ctx context.Context, wExecID core.ID) (map[core.ID]any, error)
}
