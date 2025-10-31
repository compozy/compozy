package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/logger"
)

// ExecuteWorkflow triggers an asynchronous workflow execution and returns the execution handle.
func (c *Client) ExecuteWorkflow(
	ctx context.Context,
	workflowID string,
	req *WorkflowExecuteRequest,
) (*WorkflowExecuteResponse, error) {
	id := strings.TrimSpace(workflowID)
	if id == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	path := fmt.Sprintf("%s/%s/executions", routes.Workflows(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result, err := decodeEnvelope[WorkflowExecuteResponse](resp, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "workflow", id, result.ExecID)
	return &result, nil
}

// ExecuteWorkflowSync executes a workflow and waits for completion.
func (c *Client) ExecuteWorkflowSync(
	ctx context.Context,
	workflowID string,
	req *WorkflowSyncRequest,
) (*WorkflowSyncResponse, error) {
	id := strings.TrimSpace(workflowID)
	if id == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	path := fmt.Sprintf("%s/%s/executions/sync", routes.Workflows(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result, err := decodeEnvelope[WorkflowSyncResponse](resp, http.StatusOK)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "workflow_sync", id, result.ExecID)
	return &result, nil
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
	streamPath := fmt.Sprintf("%s/workflows/%s/stream", routes.Executions(), url.PathEscape(handle.ExecID))
	return c.openStream(ctx, streamPath, nil, opts, handle.ExecID, handle.ExecURL)
}

func logExecution(ctx context.Context, kind string, resource string, execID string) {
	log := logger.FromContext(ctx)
	if log == nil {
		return
	}
	log.Info("execution triggered", "kind", kind, "resource", resource, "exec_id", execID)
}
