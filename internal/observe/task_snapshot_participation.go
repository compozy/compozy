package observe

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func taskSummaryIndex(
	tasks []taskpkg.Summary,
) (map[string]taskpkg.Summary, map[string]struct{}) {
	tasksByID := make(map[string]taskpkg.Summary, len(tasks))
	taskIDs := make(map[string]struct{}, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		taskID := strings.TrimSpace(item.ID)
		if taskID == "" {
			continue
		}
		tasksByID[taskID] = *item
		taskIDs[taskID] = struct{}{}
	}
	return tasksByID, taskIDs
}

func taskParticipationChannels(tasks []taskpkg.Summary, runs []taskpkg.Run) map[string]string {
	selectedRuns := make(map[string]taskpkg.Run, len(tasks))
	currentRunIDs := make(map[string]string, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		taskID := strings.TrimSpace(item.ID)
		if taskID == "" {
			continue
		}
		currentRunIDs[taskID] = strings.TrimSpace(item.CurrentRunID)
	}

	for idx := range runs {
		candidate := runs[idx]
		taskID := strings.TrimSpace(candidate.TaskID)
		currentRunID, knownTask := currentRunIDs[taskID]
		if !knownTask {
			continue
		}

		selected, hasSelected := selectedRuns[taskID]
		if !hasSelected || prefersTaskParticipationRun(candidate, selected, currentRunID) {
			selectedRuns[taskID] = candidate
		}
	}

	channels := make(map[string]string, len(selectedRuns))
	for taskID, selected := range selectedRuns {
		channels[taskID] = taskRunParticipationChannel(selected)
	}
	return channels
}

func taskRunParticipationChannel(run taskpkg.Run) string {
	return strings.TrimSpace(run.NetworkSpecSnapshot().ChannelID)
}

func prefersTaskParticipationRun(candidate, selected taskpkg.Run, currentRunID string) bool {
	if currentRunID != "" {
		candidateIsCurrent := strings.TrimSpace(candidate.ID) == currentRunID
		selectedIsCurrent := strings.TrimSpace(selected.ID) == currentRunID
		if candidateIsCurrent != selectedIsCurrent {
			return candidateIsCurrent
		}
	}

	candidateRank := dashboardActiveRunRank(candidate.Status)
	selectedRank := dashboardActiveRunRank(selected.Status)
	if candidateRank != selectedRank {
		return candidateRank > selectedRank
	}

	candidateActivity := dashboardRunActivityAt(candidate)
	selectedActivity := dashboardRunActivityAt(selected)
	if !candidateActivity.Equal(selectedActivity) {
		return candidateActivity.After(selectedActivity)
	}
	return strings.TrimSpace(candidate.ID) > strings.TrimSpace(selected.ID)
}

func filterTasksByNetworkChannel(
	tasks []taskpkg.Summary,
	channels map[string]string,
	channel string,
) []taskpkg.Summary {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return tasks
	}

	filtered := make([]taskpkg.Summary, 0, len(tasks))
	for idx := range tasks {
		item := &tasks[idx]
		if taskParticipationChannel(channels, item.ID) == channel {
			filtered = append(filtered, *item)
		}
	}
	return filtered
}

func taskParticipationChannel(channels map[string]string, taskID string) string {
	return strings.TrimSpace(channels[strings.TrimSpace(taskID)])
}
