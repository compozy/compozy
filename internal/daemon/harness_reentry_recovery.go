package daemon

import (
	"context"
	"fmt"
	"sort"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (b *harnessReentryBridge) loadRecoveredDetachedHarnessRuns(
	ctx context.Context,
) ([]recoveredDetachedHarnessRun, error) {
	runs, err := b.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{
		taskpkg.TaskRunStatusCompleted,
		taskpkg.TaskRunStatusFailed,
		taskpkg.TaskRunStatusCanceled,
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: list detached terminal runs for reentry recovery: %w", err)
	}

	recovered := make([]recoveredDetachedHarnessRun, 0, len(runs))
	for _, run := range runs {
		if !run.IsTaskAnchored() {
			continue
		}
		metadata, ok, err := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
		if err != nil {
			return nil, err
		}
		if !ok || detachedHarnessReentryProcessed(metadata.Reentry) {
			continue
		}
		sequence, timestamp, lookupErr := b.latestDetachedTerminalSequence(ctx, run.TaskID, run.ID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if timestamp.IsZero() {
			timestamp = run.EndedAt
		}
		recovered = append(recovered, recoveredDetachedHarnessRun{
			run:           run,
			completionSeq: sequence,
			completedAt:   timestamp,
		})
	}

	sort.SliceStable(recovered, func(i, j int) bool {
		if !recovered[i].completedAt.Equal(recovered[j].completedAt) {
			return recovered[i].completedAt.Before(recovered[j].completedAt)
		}
		if recovered[i].completionSeq != recovered[j].completionSeq {
			return recovered[i].completionSeq < recovered[j].completionSeq
		}
		return recovered[i].run.ID < recovered[j].run.ID
	})

	return recovered, nil
}
