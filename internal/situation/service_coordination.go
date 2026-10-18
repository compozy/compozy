package situation

import (
	"slices"
	"strings"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func selectActiveRun(runs []taskpkg.Run) (taskpkg.Run, bool) {
	active := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if activeRunRank(run.Status) < 0 {
			continue
		}
		active = append(active, run)
	}
	if len(active) == 0 {
		return taskpkg.Run{}, false
	}
	slices.SortStableFunc(active, func(left, right taskpkg.Run) int {
		leftRank := activeRunRank(left.Status)
		rightRank := activeRunRank(right.Status)
		if leftRank != rightRank {
			return leftRank - rightRank
		}
		if leftTime, rightTime := runActivityTime(left), runActivityTime(right); !leftTime.Equal(rightTime) {
			if leftTime.After(rightTime) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
	return active[0], true
}

func activeRunRank(status taskpkg.RunStatus) int {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusRunning:
		return 0
	case taskpkg.TaskRunStatusStarting:
		return 1
	case taskpkg.TaskRunStatusClaimed:
		return 2
	case taskpkg.TaskRunStatusQueued:
		return 3
	default:
		return -1
	}
}

func runActivityTime(run taskpkg.Run) time.Time {
	return latestTime(run.QueuedAt, run.ClaimedAt, run.StartedAt, run.EndedAt)
}
