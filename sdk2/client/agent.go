package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/routes"
)

// ExecuteAgent triggers an asynchronous agent execution.
func (c *Client) ExecuteAgent(
	ctx context.Context,
	agentID string,
	req *AgentExecuteRequest,
) (*AgentExecuteResponse, error) {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	path := fmt.Sprintf("%s/%s/executions", routes.Agents(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, nil, req)
	if err != nil {
		return nil, err
	}
	result, err := decodeEnvelope[AgentExecuteResponse](resp, http.StatusAccepted)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "agent", id, result.ExecID)
	return &result, nil
}

// ExecuteAgentSync executes an agent synchronously.
func (c *Client) ExecuteAgentSync(
	ctx context.Context,
	agentID string,
	req *AgentExecuteRequest,
) (*AgentSyncResponse, error) {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	path := fmt.Sprintf("%s/%s/executions/sync", routes.Agents(), url.PathEscape(id))
	resp, err := c.postJSON(ctx, path, nil, req)
	if err != nil {
		return nil, err
	}
	result, err := decodeEnvelope[AgentSyncResponse](resp, http.StatusOK)
	if err != nil {
		return nil, err
	}
	logExecution(ctx, "agent_sync", id, result.ExecID)
	return &result, nil
}

// ExecuteAgentStream starts an agent execution and streams results.
func (c *Client) ExecuteAgentStream(
	ctx context.Context,
	agentID string,
	req *AgentExecuteRequest,
	opts *StreamOptions,
) (*StreamSession, error) {
	handle, err := c.ExecuteAgent(ctx, agentID, req)
	if err != nil {
		return nil, err
	}
	streamPath := fmt.Sprintf("%s/agents/%s/stream", routes.Executions(), url.PathEscape(handle.ExecID))
	return c.openStream(ctx, streamPath, nil, opts, handle.ExecID, handle.ExecURL)
}
