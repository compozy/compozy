package globaldb

import (
	"context"
	"fmt"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type reservedCoordinatorWake struct {
	spec  taskpkg.CoordinatorWakeSpec
	runID string
}

func (g *TaskRepo) reserveCoordinatorPostCommitWakes(
	specs []taskpkg.CoordinatorWakeSpec,
) ([]reservedCoordinatorWake, error) {
	reserved := make([]reservedCoordinatorWake, 0, len(specs))
	for _, spec := range specs {
		runID, err := g.newID("run")
		if err != nil {
			return nil, fmt.Errorf("store: generate coordinator post-commit wake run id: %w", err)
		}
		reserved = append(reserved, reservedCoordinatorWake{spec: spec.Normalize(), runID: runID})
	}
	return reserved, nil
}

func (g *TaskRepo) enqueueCoordinatorPostCommitWakes(
	ctx context.Context,
	wakes []reservedCoordinatorWake,
	origin taskpkg.Origin,
	now time.Time,
) error {
	for _, wake := range wakes {
		if _, _, err := g.enqueueLoopCoordinatorWake(
			ctx,
			wake.spec.LoopRunID,
			wake.runID,
			wake.spec.IdempotencyKey,
			origin,
			now,
		); err != nil {
			return fmt.Errorf("store: enqueue coordinator post-commit wake %q: %w", wake.spec.LoopRunID, err)
		}
	}
	return nil
}
