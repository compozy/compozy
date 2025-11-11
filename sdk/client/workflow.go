package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/logger"
)

// ExecuteWorkflow triggers an asynchronous workflow execution and returns the execution handle.
func (c *Client) ExecuteWorkflow(
	ctx context.Context,
	workflowID string,
	req *WorkflowExecuteRequest,
) (*WorkflowExecuteResponse, error) {
	return executeRequest(ctx, c, executionRequestConfig[WorkflowExecuteResponse]{
		ResourceID:    workflowID,
		ResourceLabel: "workflow",
		RouteBase:     routes.Workflows(),
		PathSuffix:    "/executions",
		Body:          req,
		ExpectedCode:  http.StatusAccepted,
		OnSuccess: func(execCtx context.Context, id string, res *WorkflowExecuteResponse) {
			logExecution(execCtx, "workflow", id, res.ExecID)
		},
	})
}

// ExecuteWorkflowSync executes a workflow and waits for completion.
func (c *Client) ExecuteWorkflowSync(
	ctx context.Context,
	workflowID string,
	req *WorkflowSyncRequest,
) (*WorkflowSyncResponse, error) {
	return executeRequest(ctx, c, executionRequestConfig[WorkflowSyncResponse]{
		ResourceID:    workflowID,
		ResourceLabel: "workflow",
		RouteBase:     routes.Workflows(),
		PathSuffix:    "/executions/sync",
		Body:          req,
		ExpectedCode:  http.StatusOK,
		OnSuccess: func(execCtx context.Context, id string, res *WorkflowSyncResponse) {
			logExecution(execCtx, "workflow_sync", id, res.ExecID)
		},
	})
}

// ExecuteWorkflowStream starts an asynchronous workflow execution and streams events until completion.
func (c *Client) ExecuteWorkflowStream(
	ctx context.Context,
	workflowID string,
	req *WorkflowExecuteRequest,
	opts *StreamOptions,
) (*StreamSession, error) {
	handle, err := c.ExecuteWorkflow(ctx, workflowID, req)
	if err != nil {
		return nil, err
	}
	streamPath := fmt.Sprintf("%s/%s/stream", routes.Executions(), url.PathEscape(handle.ExecID))
	return c.openStream(ctx, streamPath, nil, opts, handle.ExecID, handle.ExecURL)
}

func logExecution(ctx context.Context, kind string, resource string, execID string) {
	log := logger.FromContext(ctx)
	if log == nil {
		return
	}
	log.Info("execution triggered", "kind", kind, "resource", resource, "exec_id", execID)
}
