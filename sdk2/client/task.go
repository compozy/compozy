package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/routes"
)

// ExecuteTask triggers an asynchronous direct task execution.
func (c *Client) ExecuteTask(
	ctx context.Context,
	taskID string,
	req *TaskExecuteRequest,
) (*TaskExecuteResponse, error) {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return nil, fmt.Errorf("task id is required")
	}
	path := fmt.Sprintf("%s/%s/executions", routes.Tasks(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, nil, req)
	if err != nil {
		return nil, err
	}
	result, err := decodeEnvelope[TaskExecuteResponse](resp, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "task", id, result.ExecID)
	return &result, nil
}

// ExecuteTaskSync executes a task synchronously and waits for completion.
func (c *Client) ExecuteTaskSync(
	ctx context.Context,
	taskID string,
	req *TaskExecuteRequest,
) (*TaskSyncResponse, error) {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return nil, fmt.Errorf("task id is required")
	}
	path := fmt.Sprintf("%s/%s/executions/sync", routes.Tasks(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, nil, req)
	if err != nil {
		return nil, err
	}
	result, err := decodeEnvelope[TaskSyncResponse](resp, http.StatusOK)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "task_sync", id, result.ExecID)
	return &result, nil
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
