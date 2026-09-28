package extensionpkg

import (
	"context"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (h *HostAPIHandler) taskRunDetailPayloadFromView(
	ctx context.Context,
	view *taskpkg.RunDetailView,
) (apicontract.TaskRunDetailPayload, error) {
	if view == nil {
		return apicontract.TaskRunDetailPayload{}, nil
	}

	var task *apicontract.TaskReferencePayload
	if view.Task != nil {
		payload := taskReferencePayloadFromReference(*view.Task)
		task = &payload
	}

	payload := apicontract.TaskRunDetailPayload{
		Run:     taskRunPayloadFromRun(&view.Run),
		Task:    task,
		Session: taskRunSessionPayloadFromSession(view.Session),
		Summary: taskRunOperationalSummaryPayloadFromSummary(view.Summary),
	}
	runs := []apicontract.TaskRunPayload{payload.Run}
	if err := h.decorateTaskRuns(ctx, runs); err != nil {
		return apicontract.TaskRunDetailPayload{}, err
	}
	payload.Run = runs[0]
	return payload, nil
}
