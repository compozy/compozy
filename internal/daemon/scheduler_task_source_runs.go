package daemon

import (
	"context"
	"fmt"

	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s schedulerTaskSource) PendingRuns(ctx context.Context) ([]schedulerpkg.RunSnapshot, error) {
	runs, err := s.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		return nil, err
	}
	workerRuns := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if !run.IsTaskAnchored() {
			continue
		}
		if run.RunKind.Normalize() == taskpkg.RunKindCoordinator {
			continue
		}
		if isQueuedLoopActionRun(run) {
			continue
		}
		workerRuns = append(workerRuns, run)
	}
	snapshots, err := s.joinRunsWithTasks(ctx, workerRuns)
	if err != nil {
		return nil, err
	}
	return s.filterPausedRuns(ctx, snapshots)
}

func (s schedulerTaskSource) ActiveRuns(ctx context.Context) ([]taskpkg.Run, error) {
	runs, err := s.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	})
	if err != nil {
		return nil, err
	}
	active := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if run.IsTaskAnchored() || run.IsNetworkWake() {
			active = append(active, run)
		}
	}
	return active, nil
}

func (s schedulerTaskSource) joinRunsWithTasks(
	ctx context.Context,
	runs []taskpkg.Run,
) ([]schedulerpkg.RunSnapshot, error) {
	work := make([]schedulerpkg.RunSnapshot, 0, len(runs))
	for _, run := range runs {
		taskRecord, err := s.store.GetTask(ctx, run.TaskID)
		if err != nil {
			return nil, fmt.Errorf("daemon: scheduler load task %q for run %q: %w", run.TaskID, run.ID, err)
		}
		if taskRecord.Paused {
			continue
		}
		if pauseReader, ok := s.store.(interface {
			IsTaskEffectivelyPaused(context.Context, string) (bool, string, error)
		}); ok {
			paused, _, err := pauseReader.IsTaskEffectivelyPaused(ctx, taskRecord.ID)
			if err != nil {
				return nil, err
			}
			if paused {
				continue
			}
		}
		work = append(work, schedulerpkg.RunSnapshot{Task: taskRecord, Run: run})
	}
	return work, nil
}

func (s schedulerTaskSource) filterPausedRuns(
	ctx context.Context,
	snapshots []schedulerpkg.RunSnapshot,
) ([]schedulerpkg.RunSnapshot, error) {
	pauseStore, ok := s.store.(effectiveTaskPauseStore)
	if !ok {
		return snapshots, nil
	}
	filtered := make([]schedulerpkg.RunSnapshot, 0, len(snapshots))
	for idx := range snapshots {
		snapshot := &snapshots[idx]
		paused, _, err := pauseStore.IsTaskEffectivelyPaused(ctx, snapshot.Task.ID)
		if err != nil {
			return nil, fmt.Errorf("daemon: scheduler check task %q pause: %w", snapshot.Task.ID, err)
		}
		if paused {
			continue
		}
		filtered = append(filtered, *snapshot)
	}
	return filtered, nil
}
