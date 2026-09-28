package observe

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (o *Observer) countActiveOrphanRuns(ctx context.Context, runs []taskpkg.Run) (int, error) {
	liveSessions, err := o.liveSessionIDs(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range runs {
		status := item.Status.Normalize()
		if status != taskpkg.TaskRunStatusStarting && status != taskpkg.TaskRunStatusRunning {
			continue
		}
		sessionID := strings.TrimSpace(item.SessionID)
		if sessionID == "" {
			count++
			continue
		}
		if _, ok := liveSessions[sessionID]; !ok {
			count++
		}
	}
	return count, nil
}

func (o *Observer) liveSessionIDs(ctx context.Context) (map[string]struct{}, error) {
	live := make(map[string]struct{})
	if o.sessionSource != nil {
		for _, info := range o.sessionSource.List() {
			if info == nil || !isLiveSessionState(string(info.State)) {
				continue
			}
			live[strings.TrimSpace(info.ID)] = struct{}{}
		}
		return live, nil
	}

	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
	})
	if err != nil {
		return nil, fmt.Errorf("observe: list sessions for task health: %w", err)
	}
	for _, info := range sessions {
		if !isLiveSessionState(info.State) {
			continue
		}
		live[strings.TrimSpace(info.ID)] = struct{}{}
	}
	return live, nil
}

func isLiveSessionState(state string) bool {
	normalized := strings.TrimSpace(state)
	return normalized != "" && normalized != string(session.StateStopped) && normalized != sessionStateOrphaned
}

func filterTasksByOrigin(tasks []taskpkg.Summary, origin taskpkg.OriginKind) []taskpkg.Summary {
	normalizedOrigin := origin.Normalize()
	if normalizedOrigin == "" {
		return tasks
	}
	filtered := make([]taskpkg.Summary, 0, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		if item.Origin.Kind.Normalize() == normalizedOrigin {
			filtered = append(filtered, *item)
		}
	}
	return filtered
}

func filterTasksByCreatedBy(
	tasks []taskpkg.Summary,
	excluded []taskpkg.ActorRef,
) []taskpkg.Summary {
	if len(excluded) == 0 {
		return tasks
	}
	filtered := make([]taskpkg.Summary, 0, len(tasks))
	for index := range tasks {
		item := tasks[index]
		shouldExclude := false
		for _, actor := range excluded {
			if item.CreatedBy.Kind.Normalize() == actor.Kind.Normalize() &&
				strings.TrimSpace(item.CreatedBy.Ref) == strings.TrimSpace(actor.Ref) {
				shouldExclude = true
				break
			}
		}
		if !shouldExclude {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterTasksByWorktree(
	tasks []taskpkg.Summary,
	runs []taskpkg.Run,
	worktreeID string,
) []taskpkg.Summary {
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return tasks
	}

	matchingTaskIDs := make(map[string]struct{}, len(tasks))
	for idx := range runs {
		candidate := runs[idx]
		if dashboardActiveRunRank(candidate.Status) == 0 ||
			strings.TrimSpace(candidate.WorktreeIDValue()) != worktreeID {
			continue
		}
		matchingTaskIDs[strings.TrimSpace(candidate.TaskID)] = struct{}{}
	}

	filtered := make([]taskpkg.Summary, 0, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		if _, ok := matchingTaskIDs[strings.TrimSpace(item.ID)]; ok {
			filtered = append(filtered, *item)
		}
	}
	return filtered
}

func filterRunsByWorktree(runs []taskpkg.Run, worktreeID string) []taskpkg.Run {
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		return runs
	}
	filtered := make([]taskpkg.Run, 0, len(runs))
	for idx := range runs {
		if strings.TrimSpace(runs[idx].WorktreeIDValue()) == worktreeID {
			filtered = append(filtered, runs[idx])
		}
	}
	return filtered
}

func filterRuns(runs []taskpkg.Run, taskIDs map[string]struct{}, query TaskSummaryQuery) []taskpkg.Run {
	normalizedOrigin := query.OriginKind.Normalize()
	filtered := make([]taskpkg.Run, 0, len(runs))
	for _, item := range runs {
		if _, ok := taskIDs[strings.TrimSpace(item.TaskID)]; !ok {
			continue
		}
		if normalizedOrigin != "" && item.Origin.Kind.Normalize() != normalizedOrigin {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterRunsByOrigin(runs []taskpkg.Run, origin taskpkg.OriginKind) []taskpkg.Run {
	normalizedOrigin := origin.Normalize()
	if normalizedOrigin == "" {
		return runs
	}
	filtered := make([]taskpkg.Run, 0, len(runs))
	for _, item := range runs {
		if item.Origin.Kind.Normalize() == normalizedOrigin {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterEventsForTasks(
	events []taskpkg.Event,
	taskIDs map[string]struct{},
	runIDs map[string]taskpkg.Run,
	worktreeID string,
) []taskpkg.Event {
	filtered := make([]taskpkg.Event, 0, len(events))
	for _, item := range events {
		if _, ok := taskIDs[strings.TrimSpace(item.TaskID)]; !ok {
			continue
		}
		if strings.TrimSpace(worktreeID) != "" && strings.TrimSpace(item.RunID) != "" {
			if _, ok := runIDs[strings.TrimSpace(item.RunID)]; !ok {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterTaskEvents(
	events []taskpkg.Event,
	query TaskMetricsQuery,
) []taskpkg.Event {
	normalizedOrigin := query.OriginKind.Normalize()
	var filtered []taskpkg.Event
	for i, item := range events {
		accepted := true
		if !query.Since.IsZero() && item.Timestamp.Before(query.Since) {
			accepted = false
		}
		if accepted && normalizedOrigin != "" && item.Origin.Kind.Normalize() != normalizedOrigin {
			accepted = false
		}
		if accepted {
			if filtered != nil {
				filtered = append(filtered, item)
			}
			continue
		}
		if filtered == nil {
			filtered = make([]taskpkg.Event, 0, len(events)-1)
			filtered = append(filtered, events[:i]...)
		}
	}
	if filtered == nil {
		return events
	}
	return filtered
}
