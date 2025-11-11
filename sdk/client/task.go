package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/engine/infra/server/routes"
)

// ExecuteTask triggers an asynchronous direct task execution.
func (c *Client) ExecuteTask(
	ctx context.Context,
	taskID string,
	req *TaskExecuteRequest,
) (*TaskExecuteResponse, error) {
	return executeRequest(ctx, c, executionRequestConfig[TaskExecuteResponse]{
		ResourceID:    taskID,
		ResourceLabel: "task",
		RouteBase:     routes.Tasks(),
		PathSuffix:    "/executions",
		Body:          req,
		ExpectedCode:  http.StatusAccepted,
		OnSuccess: func(execCtx context.Context, id string, res *TaskExecuteResponse) {
			logExecution(execCtx, "task", id, res.ExecID)
		},
	})
}

// ExecuteTaskSync executes a task synchronously and waits for completion.
func (c *Client) ExecuteTaskSync(
	ctx context.Context,
	taskID string,
	req *TaskExecuteRequest,
) (*TaskSyncResponse, error) {
	return executeRequest(ctx, c, executionRequestConfig[TaskSyncResponse]{
		ResourceID:    taskID,
		ResourceLabel: "task",
		RouteBase:     routes.Tasks(),
		PathSuffix:    "/executions/sync",
		Body:          req,
		ExpectedCode:  http.StatusOK,
		OnSuccess: func(execCtx context.Context, id string, res *TaskSyncResponse) {
			logExecution(execCtx, "task_sync", id, res.ExecID)
		},
	})
}

// ExecuteTaskStream starts a task execution and streams events until completion.
func (c *Client) ExecuteTaskStream(
	ctx context.Context,
	taskID string,
	req *TaskExecuteRequest,
	opts *StreamOptions,
) (*StreamSession, error) {
	handle, err := c.ExecuteTask(ctx, taskID, req)
	if err != nil {
		return nil, err
	}
	streamPath := fmt.Sprintf("%s/tasks/%s/stream", routes.Executions(), url.PathEscape(handle.ExecID))
	return c.openStream(ctx, streamPath, nil, opts, handle.ExecID, handle.ExecURL)
}
