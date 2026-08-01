package globaldb

import (
	"fmt"
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func validateCoordinatorSnapshotLoopIdentity(
	plan taskpkg.CoordinatorCompletionPlan,
	loopRunID string,
) error {
	want := strings.TrimSpace(loopRunID)
	if want == "" {
		return fmt.Errorf("%w: claimed coordinator loop_run_id is required", taskpkg.ErrValidation)
	}
	for _, candidate := range []struct {
		name     string
		snapshot *taskpkg.GenerationSnapshot
	}{
		{name: "snapshot", snapshot: &plan.Snapshot},
		{name: "post-reserve snapshot", snapshot: plan.PostReserveSnapshot},
	} {
		if candidate.snapshot == nil {
			continue
		}
		got := strings.TrimSpace(candidate.snapshot.LoopRunID)
		if got != "" && got != want {
			return fmt.Errorf(
				"%w: coordinator %s loop_run_id %q does not match claimed loop %q",
				taskpkg.ErrValidation,
				candidate.name,
				got,
				want,
			)
		}
	}
	return nil
}

func normalizePostReserveSnapshot(
	postReserveSnapshot *taskpkg.GenerationSnapshot,
	base taskpkg.GenerationSnapshot,
	loopRunID string,
) *taskpkg.GenerationSnapshot {
	if postReserveSnapshot == nil {
		return nil
	}
	snapshot := *postReserveSnapshot
	if strings.TrimSpace(snapshot.LoopRunID) == "" {
		snapshot.LoopRunID = loopRunID
	}
	if snapshot.Generation <= 0 {
		snapshot.Generation = base.Generation
	}
	return &snapshot
}
