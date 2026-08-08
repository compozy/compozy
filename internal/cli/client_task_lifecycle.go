package cli

import (
	"context"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) PublishTask(
	ctx context.Context,
	id string,
	request TaskExecutionRequest,
) (TaskExecutionRecord, error) {
	return c.taskExecutionAction(ctx, strings.TrimSpace(id), "publish", request)
}

func (c *daemonClient) StartTask(
	ctx context.Context,
	id string,
	request TaskExecutionRequest,
) (TaskExecutionRecord, error) {
	return c.taskExecutionAction(ctx, strings.TrimSpace(id), "start", request)
}

func (c *daemonClient) ApproveTask(
	ctx context.Context,
	id string,
	request TaskExecutionRequest,
) (TaskExecutionRecord, error) {
	return c.taskExecutionAction(ctx, strings.TrimSpace(id), "approve", request)
}

func (c *daemonClient) DeleteTask(ctx context.Context, id string) error {
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *daemonClient) RejectTask(ctx context.Context, id string) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/reject"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) CancelTask(ctx context.Context, id string, request CancelTaskRequest) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/cancel"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) PauseTask(ctx context.Context, id string, request PauseTaskRequest) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/pause"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) ResumeTask(
	ctx context.Context,
	id string,
	request ResumeTaskRequest,
) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/resume"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) CreateChildTask(
	ctx context.Context,
	id string,
	request CreateTaskChildRequest,
) (TaskRecord, error) {
	var response contract.TaskResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/children"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) AddTaskDependency(
	ctx context.Context,
	id string,
	request AddTaskDependencyRequest,
) (TaskDetailRecord, error) {
	var response contract.TaskDetailResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/dependencies"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskDetailRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) RemoveTaskDependency(
	ctx context.Context,
	id string,
	dependsOnID string,
) (TaskDetailRecord, error) {
	var response contract.TaskDetailResponse
	path := "/api/tasks/" + url.PathEscape(
		strings.TrimSpace(id),
	) + "/dependencies/" + url.PathEscape(
		strings.TrimSpace(dependsOnID),
	)
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, &response); err != nil {
		return TaskDetailRecord{}, err
	}
	return response.Task, nil
}

func (c *daemonClient) EnqueueTaskRun(
	ctx context.Context,
	id string,
	request EnqueueTaskRunRequest,
) (TaskRunRecord, error) {
	var response contract.TaskRunResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/runs"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return TaskRunRecord{}, err
	}
	return response.Run, nil
}

func (c *daemonClient) FanOutTaskRuns(
	ctx context.Context,
	id string,
	request FanOutTaskRunsRequest,
) (FanOutTaskRunsRecord, error) {
	var response contract.FanOutTaskRunsResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/runs/fan-out"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return FanOutTaskRunsRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ListTaskRuns(
	ctx context.Context,
	id string,
	query TaskRunListQuery,
) ([]TaskRunRecord, error) {
	var response contract.TaskRunsResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(id)) + "/runs"
	if err := c.doJSON(ctx, http.MethodGet, path, taskRunValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

func (c *daemonClient) GetTaskRun(ctx context.Context, id string) (TaskRunDetailRecord, error) {
	var response contract.TaskRunDetailResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TaskRunDetailRecord{}, err
	}
	return response.Run, nil
}

func (c *daemonClient) StartTaskRun(
	ctx context.Context,
	id string,
	request StartTaskRunRequest,
) (TaskRunRecord, error) {
	return c.taskRunAction(ctx, strings.TrimSpace(id), "start", request)
}

func (c *daemonClient) AttachTaskRunSession(
	ctx context.Context,
	id string,
	request AttachTaskRunSessionRequest,
) (TaskRunRecord, error) {
	return c.taskRunAction(ctx, strings.TrimSpace(id), "attach-session", request)
}

func (c *daemonClient) CompleteTaskRun(
	ctx context.Context,
	id string,
	request CompleteTaskRunRequest,
) (TaskRunRecord, error) {
	return c.taskRunAction(ctx, strings.TrimSpace(id), "complete", request)
}

func (c *daemonClient) FailTaskRun(
	ctx context.Context,
	id string,
	request FailTaskRunRequest,
) (TaskRunRecord, error) {
	return c.taskRunAction(ctx, strings.TrimSpace(id), "fail", request)
}

func (c *daemonClient) CancelTaskRun(
	ctx context.Context,
	id string,
	request CancelTaskRunRequest,
) (TaskRunRecord, error) {
	return c.taskRunAction(ctx, strings.TrimSpace(id), "cancel", request)
}

func (c *daemonClient) ForceReleaseTaskRun(
	ctx context.Context,
	id string,
	request ForceReleaseTaskRunRequest,
) (TaskRunRecord, error) {
	return c.forceTaskRunAction(ctx, strings.TrimSpace(id), "release", request)
}

func (c *daemonClient) ForceFailTaskRun(
	ctx context.Context,
	id string,
	request ForceFailTaskRunRequest,
) (TaskRunRecord, error) {
	return c.forceTaskRunAction(ctx, strings.TrimSpace(id), "fail", request)
}

func (c *daemonClient) RetryTaskRun(
	ctx context.Context,
	id string,
	request RetryTaskRunRequest,
) (RetryTaskRunRecord, error) {
	var response contract.RetryTaskRunResponse
	path := "/api/runs/" + url.PathEscape(strings.TrimSpace(id)) + "/retry"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return RetryTaskRunRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) RecoverTaskRun(
	ctx context.Context,
	id string,
	request RecoverTaskRunRequest,
) (RetryTaskRunRecord, error) {
	var response contract.RetryTaskRunResponse
	path := "/api/runs/" + url.PathEscape(strings.TrimSpace(id)) + "/recover"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return RetryTaskRunRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) BulkForceReleaseTaskRuns(
	ctx context.Context,
	request BulkForceTaskRunRequest,
) (BulkForceTaskRunRecord, error) {
	return c.bulkForceTaskRunAction(ctx, "release", request)
}

func (c *daemonClient) BulkForceFailTaskRuns(
	ctx context.Context,
	request BulkForceTaskRunRequest,
) (BulkForceTaskRunRecord, error) {
	return c.bulkForceTaskRunAction(ctx, "fail", request)
}

func (c *daemonClient) SchedulerStatus(ctx context.Context) (SchedulerStatusRecord, error) {
	var response contract.SchedulerStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/scheduler", nil, nil, &response); err != nil {
		return SchedulerStatusRecord{}, err
	}
	return response.Scheduler, nil
}

func (c *daemonClient) PauseScheduler(
	ctx context.Context,
	request SchedulerPauseRequest,
) (SchedulerStatusRecord, error) {
	var response contract.SchedulerStatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/scheduler/pause", nil, request, &response); err != nil {
		return SchedulerStatusRecord{}, err
	}
	return response.Scheduler, nil
}

func (c *daemonClient) ResumeScheduler(
	ctx context.Context,
	request SchedulerResumeRequest,
) (SchedulerStatusRecord, error) {
	var response contract.SchedulerStatusResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/scheduler/resume", nil, request, &response); err != nil {
		return SchedulerStatusRecord{}, err
	}
	return response.Scheduler, nil
}

func (c *daemonClient) DrainScheduler(
	ctx context.Context,
	request SchedulerDrainRequest,
) (SchedulerDrainRecord, error) {
	var response contract.SchedulerDrainResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/scheduler/drain", nil, request, &response); err != nil {
		return SchedulerDrainRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) SchedulerBacklog(
	ctx context.Context,
	query SchedulerBacklogQuery,
) (SchedulerBacklogRecord, error) {
	var response contract.SchedulerBacklogResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/scheduler/backlog",
		schedulerBacklogValues(query),
		nil,
		&response,
	); err != nil {
		return SchedulerBacklogRecord{}, err
	}
	return response.Backlog, nil
}
