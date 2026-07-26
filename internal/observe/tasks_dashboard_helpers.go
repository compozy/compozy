package observe

import (
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"

	taskpkg "github.com/compozy/agh/internal/task"
)

func countTaskStatus(rows []TaskStatusTotal, status taskpkg.Status) int {
	count := 0
	for _, item := range rows {
		if item.Status.Normalize() == status.Normalize() {
			count += item.Count
		}
	}
	return count
}

func countRunStatus(rows []TaskRunTotal, status taskpkg.RunStatus) int {
	count := 0
	for _, item := range rows {
		if item.Status.Normalize() == status.Normalize() {
			count += item.Count
		}
	}
	return count
}

func countAwaitingApprovalTasks(tasks []taskpkg.Summary) int {
	count := 0
	for idx := range tasks {
		item := &tasks[idx]
		if item.Status.Normalize() == taskpkg.TaskStatusBlocked &&
			item.ApprovalState.Normalize() == taskpkg.ApprovalStatePending {
			count++
		}
	}
	return count
}

func countDependencyBlockedTasks(tasks []taskpkg.Summary) int {
	count := 0
	for idx := range tasks {
		item := &tasks[idx]
		if item.Status.Normalize() != taskpkg.TaskStatusBlocked {
			continue
		}
		if item.DependencyCount <= 0 && len(item.Dependencies) == 0 {
			continue
		}
		count++
	}
	return count
}

func dashboardStatusForCount(count int) string {
	return dashboardStatusForAny(count > 0)
}

func dashboardStatusForAny(warn bool) string {
	if warn {
		return taskHealthStatusWarn
	}
	return taskHealthStatusOK
}

func dashboardActiveRunRank(status taskpkg.RunStatus) int {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusRunning:
		return 4
	case taskpkg.TaskRunStatusStarting:
		return 3
	case taskpkg.TaskRunStatusClaimed:
		return 2
	case taskpkg.TaskRunStatusQueued:
		return 1
	default:
		return 0
	}
}

func dashboardRunActivityAt(run taskpkg.Run) time.Time {
	latest := run.QueuedAt
	for _, candidate := range []time.Time{run.ClaimedAt, run.StartedAt, run.EndedAt} {
		if candidate.After(latest) {
			latest = candidate
		}
	}
	return latest
}

func dashboardRunAge(run taskpkg.Run, now time.Time) time.Duration {
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusRunning:
		if !run.StartedAt.IsZero() {
			return safeSince(now, run.StartedAt)
		}
	case taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusClaimed:
		if !run.ClaimedAt.IsZero() {
			return safeSince(now, run.ClaimedAt)
		}
	}
	return safeSince(now, run.QueuedAt)
}

func latestTaskSnapshotActivityAt(snapshot taskSnapshot) time.Time {
	var latest time.Time
	for idx := range snapshot.tasks {
		item := &snapshot.tasks[idx]
		for _, candidate := range []time.Time{item.CreatedAt, item.UpdatedAt, item.ClosedAt} {
			if candidate.After(latest) {
				latest = candidate
			}
		}
	}
	for _, item := range snapshot.runs {
		if activityAt := dashboardRunActivityAt(item); activityAt.After(latest) {
			latest = activityAt
		}
	}
	for _, item := range snapshot.events {
		if item.Timestamp.After(latest) {
			latest = item.Timestamp
		}
	}
	return latest
}

func snapshotHasLiveWork(snapshot taskSnapshot) bool {
	for _, item := range snapshot.runs {
		if isDashboardActiveRunStatus(item.Status) {
			return true
		}
	}
	return false
}

func dashboardNow(now func() time.Time) time.Time {
	if now == nil {
		return time.Now().UTC()
	}
	return now().UTC()
}

func dashboardActiveRuns(runs []taskpkg.Run) []taskpkg.Run {
	items := make([]taskpkg.Run, 0, len(runs))
	for _, item := range runs {
		if isDashboardActiveRunStatus(item.Status) {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, compareDashboardActiveRuns)
	return items
}

func compareDashboardActiveRuns(left, right taskpkg.Run) int {
	leftRank := dashboardActiveRunRank(left.Status)
	rightRank := dashboardActiveRunRank(right.Status)
	if leftRank != rightRank {
		if leftRank > rightRank {
			return -1
		}
		return 1
	}

	leftAt := dashboardRunActivityAt(left)
	rightAt := dashboardRunActivityAt(right)
	if !leftAt.Equal(rightAt) {
		if leftAt.After(rightAt) {
			return -1
		}
		return 1
	}
	return strings.Compare(right.ID, left.ID)
}

func dashboardStuckRunSet(
	runs []taskpkg.Run,
	currentTime time.Time,
	healthCfg TaskHealthConfig,
) map[string]struct{} {
	stuckByID := make(map[string]struct{})
	for _, item := range findStuckRuns(runs, currentTime, healthCfg) {
		stuckByID[item.RunID] = struct{}{}
	}
	return stuckByID
}

func taskDashboardActiveRunCounts(items []taskpkg.Run) TaskDashboardActiveRuns {
	activeRuns := TaskDashboardActiveRuns{Total: len(items)}
	for _, item := range items {
		switch item.Status.Normalize() {
		case taskpkg.TaskRunStatusRunning:
			activeRuns.Running++
		case taskpkg.TaskRunStatusStarting:
			activeRuns.Starting++
		case taskpkg.TaskRunStatusClaimed:
			activeRuns.Claimed++
		case taskpkg.TaskRunStatusQueued:
			activeRuns.Queued++
		}
	}
	return activeRuns
}

func taskDashboardActiveRunItems(
	items []taskpkg.Run,
	tasksByID map[string]taskpkg.Summary,
	currentTime time.Time,
	limit int,
	stuckByID map[string]struct{},
) []TaskDashboardActiveRun {
	if limit <= 0 {
		limit = 4
	}
	if limit > len(items) {
		limit = len(items)
	}

	activeRunItems := make([]TaskDashboardActiveRun, 0, limit)
	for _, run := range items[:limit] {
		taskItem, ok := tasksByID[strings.TrimSpace(run.TaskID)]
		if !ok {
			continue
		}
		_, stuck := stuckByID[run.ID]
		activeRunItems = append(activeRunItems, TaskDashboardActiveRun{
			TaskID:                       taskItem.ID,
			TaskIdentifier:               taskItem.Identifier,
			TaskTitle:                    taskItem.Title,
			TaskStatus:                   taskItem.Status.Normalize(),
			TaskPriority:                 taskItem.Priority,
			TaskOwner:                    cloneOwnership(taskItem.Owner),
			Scope:                        taskItem.Scope.Normalize(),
			WorkspaceID:                  taskItem.WorkspaceID,
			LatestEventSeq:               taskItem.LatestEventSeq,
			RunID:                        run.ID,
			RunStatus:                    run.Status.Normalize(),
			Attempt:                      int(run.Attempt),
			MaxAttempts:                  taskItem.MaxAttempts,
			SessionID:                    strings.TrimSpace(run.SessionID),
			ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
			LastActivityAt:               dashboardRunActivityAt(run),
			AgeMilli:                     dashboardRunAge(run, currentTime).Milliseconds(),
			HealthStatus:                 dashboardStatusForAny(stuck),
			Stuck:                        stuck,
			Error:                        strings.TrimSpace(run.Error),
		})
	}
	return activeRunItems
}

func isDashboardActiveRunStatus(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return true
	default:
		return false
	}
}

func cloneOwnership(owner *taskpkg.Ownership) *taskpkg.Ownership {
	if owner == nil {
		return nil
	}
	cloned := *owner
	return &cloned
}
