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
	LoadExecution(ctx context.Context, wExecID core.ID, taskExecID core.ID) (*Execution, error)
	LoadExecutionsJSON(ctx context.Context, wExecID core.ID) (map[core.ID]core.JSONMap, error)
}
