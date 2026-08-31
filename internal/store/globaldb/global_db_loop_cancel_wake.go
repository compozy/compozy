package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *LoopRepo) reserveCancellationCoordinator(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	mutation looppkg.CancellationMutation,
) (*taskpkg.Run, error) {
	coordinator, err := g.reserveOrReuseOpenLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		run,
		cancellationWakeOrigin(),
		mutation.RequestedAt.UTC(),
		cancellationWakeIdempotencyKey(mutation),
	)
	if err != nil {
		return nil, err
	}
	return &coordinator, nil
}

func cancellationWakeOrigin() taskpkg.Origin {
	return taskpkg.Origin{
		Kind: taskpkg.OriginKindDaemon,
		Ref:  "loop.cancellation:cancel",
	}
}

func cancellationWakeIdempotencyKey(mutation looppkg.CancellationMutation) string {
	target := "run:" + string(mutation.RunID)
	if mutation.NodeID != "" {
		target = "node:" + string(mutation.NodeID)
	}
	return fmt.Sprintf(
		"loop.cancel.%s.%s.%s.%d",
		mutation.RunID,
		target,
		"cancel",
		mutation.RequestedAt.UTC().UnixNano(),
	)
}
