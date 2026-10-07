package daemon

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
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
		if strings.TrimSpace(run.TaskID) == "" {
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

	slices.SortStableFunc(recovered, func(a, b recoveredDetachedHarnessRun) int {
		return cmp.Or(
			a.completedAt.Compare(b.completedAt),
			cmp.Compare(a.completionSeq, b.completionSeq),
			strings.Compare(a.run.ID, b.run.ID),
		)
	})

	return recovered, nil
}
