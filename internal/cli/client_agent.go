package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// AgentRecord is the shared daemon agent definition payload.
type AgentRecord = contract.AgentPayload

// AgentMCPServer is one MCP server entry returned by the daemon API.
type AgentMCPServer = contract.AgentMCPServerJSON

// AgentQuery captures agent definition filters.
type AgentQuery struct {
	Workspace string
}

func (c *unixSocketClient) ListAgents(ctx context.Context, query AgentQuery) ([]AgentRecord, error) {
	var response contract.AgentsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/agents", agentValues(query), nil, &response); err != nil {
		return nil, fmt.Errorf("cli: list agents: %w", err)
	}
	return response.Agents, nil
}

func (c *unixSocketClient) GetAgent(ctx context.Context, name string, query AgentQuery) (AgentRecord, error) {
	var response contract.AgentResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		agentPath(name),
		agentValues(query),
		nil,
		&response,
	); err != nil {
		return AgentRecord{}, fmt.Errorf("cli: get agent: %w", err)
	}
	return response.Agent, nil
}

func (c *unixSocketClient) CreateAgent(
	ctx context.Context,
	request contract.CreateAgentRequest,
) (AgentRecord, error) {
	var response contract.AgentResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/agents", nil, request, &response); err != nil {
		return AgentRecord{}, fmt.Errorf("cli: create agent: %w", err)
	}
	return response.Agent, nil
}

func (c *unixSocketClient) UpdateAgent(
	ctx context.Context,
	name string,
	request contract.UpdateAgentRequest,
) (AgentRecord, error) {
	var response contract.AgentResponse
	if err := c.doJSON(ctx, http.MethodPut, agentPath(name), nil, request, &response); err != nil {
		return AgentRecord{}, fmt.Errorf("cli: update agent: %w", err)
	}
	return response.Agent, nil
}

func (c *unixSocketClient) DeleteAgent(
	ctx context.Context,
	name string,
	workspace string,
) (contract.DeleteAgentResponse, error) {
	var response contract.DeleteAgentResponse
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		agentPath(name),
		agentValues(AgentQuery{Workspace: workspace}),
		nil,
		&response,
	); err != nil {
		return contract.DeleteAgentResponse{}, fmt.Errorf("cli: delete agent: %w", err)
	}
	return response, nil
}

func (c *unixSocketClient) DuplicateAgent(
	ctx context.Context,
	name string,
	request contract.DuplicateAgentRequest,
) (AgentRecord, error) {
	var response contract.AgentResponse
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		agentPath(name)+"/duplicate",
		nil,
		request,
		&response,
	); err != nil {
		return AgentRecord{}, fmt.Errorf("cli: duplicate agent: %w", err)
	}
	return response.Agent, nil
}

func agentPath(name string) string {
	return "/api/agents/" + url.PathEscape(strings.TrimSpace(name))
}

func agentValues(query AgentQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	return values
}
