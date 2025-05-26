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
	LoadExecution(ctx context.Context, wExecID core.ID, toolExecID core.ID) (*Execution, error)
	LoadExecutionsJSON(ctx context.Context, wExecID core.ID) (map[core.ID]core.JSONMap, error)
}
