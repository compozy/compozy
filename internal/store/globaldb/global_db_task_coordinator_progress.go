package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRepo) reserveCoordinatorConcurrentProgressWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	completion taskpkg.CoordinatorCompletion,
	current taskpkg.Run,
	result *taskpkg.CoordinatorCompletionResult,
) error {
	if !completion.Plan.GenerationInFlight || len(completion.Plan.PostCommitWakes) > 0 || current.QueuedAt.IsZero() {
		return nil
	}
	loopRun, err := getLoopRunByIDWithExecutor(ctx, exec, loop.RunID(strings.TrimSpace(current.LoopRunID)))
	if err != nil {
		return err
	}
	if loopRun.Status != loop.StatusRunning || !loopRun.LastProgressAt.After(current.QueuedAt) {
		return nil
	}
	openRunID, err := g.findOpenRunIDForQueuedRunReservation(ctx, exec, current.TaskID, "")
	if err != nil || openRunID != "" {
		return err
	}
	reservation := queuedRunReservationInput{
		taskID:         current.TaskID,
		runID:          store.NewID("run"),
		runKind:        taskpkg.RunKindCoordinator,
		loopRunID:      current.LoopRunID,
		idempotencyKey: coordinatorConcurrentProgressWakeKey(current.LoopRunID, current.ID),
		origin:         completion.Actor.Origin,
		networkSpec:    current.NetworkSpecSnapshot(),
		queuedAt:       completion.Now,
	}
	_, run, existing, err := g.reserveQueuedRunWithExecutor(ctx, exec, reservation)
	if err != nil {
		return fmt.Errorf("store: reserve concurrent-progress coordinator wake: %w", err)
	}
	if !existing {
		result.EnqueuedRuns = append(result.EnqueuedRuns, run)
	}
	return nil
}

func coordinatorConcurrentProgressWakeKey(loopRunID string, coordinatorRunID string) string {
	return fmt.Sprintf(
		"loop.coordinator.concurrent_progress.%s.%s",
		strings.TrimSpace(loopRunID),
		strings.TrimSpace(coordinatorRunID),
	)
}
