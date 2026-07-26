package core

import (
	"github.com/compozy/agh/internal/api/contract"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskRunDetailPayloadFromView converts one run-detail view into the shared payload.
func TaskRunDetailPayloadFromView(view *taskpkg.RunDetailView) contract.TaskRunDetailPayload {
	if view == nil {
		return contract.TaskRunDetailPayload{}
	}

	var task *contract.TaskReferencePayload
	if view.Task != nil {
		payload := TaskReferencePayloadFromReference(*view.Task)
		task = &payload
	}

	return contract.TaskRunDetailPayload{
		Run:     TaskRunPayloadFromRun(&view.Run),
		Task:    task,
		Session: TaskRunSessionPayloadFromSession(view.Session),
		Summary: TaskRunOperationalSummaryPayloadFromSummary(view.Summary),
	}
}
