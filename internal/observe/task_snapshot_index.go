package observe

import (
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
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
