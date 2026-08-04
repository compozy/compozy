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
		cancellationWakeOrigin(mutation),
		mutation.RequestedAt.UTC(),
		cancellationWakeIdempotencyKey(mutation),
	)
	if err != nil {
		return nil, err
	}
	return &coordinator, nil
}

func cancellationWakeOrigin(mutation looppkg.CancellationMutation) taskpkg.Origin {
	return taskpkg.Origin{
		Kind: taskpkg.OriginKindDaemon,
		Ref:  "loop.cancellation:" + mutation.Kind.String(),
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
		mutation.Kind.String(),
		mutation.RequestedAt.UTC().UnixNano(),
	)
}
