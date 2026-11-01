package cli

import (
	"context"
	"github.com/compozy/compozy/internal/api/contract"
	"net/http"
	"net/url"
)

type agentInstancesClient interface {
	ListAgentInstances(context.Context, url.Values) (contract.AgentInstanceListResponse, error)
	CreateAgentInstance(context.Context, contract.AgentInstanceCreateRequest) (contract.AgentInstanceResponse, error)
	AdoptAgentInstance(context.Context, contract.AgentInstanceAdoptRequest) (contract.AgentInstanceResponse, error)
	MutateAgentInstance(context.Context, string, string) (contract.AgentInstanceResponse, error)
	AgentInstanceReports(context.Context, string, url.Values) (contract.AgentInstanceReportsResponse, error)
}

var _ agentInstancesClient = (*daemonClient)(nil)

func (c *daemonClient) ListAgentInstances(ctx context.Context, query url.Values) (contract.AgentInstanceListResponse, error) {
	var result contract.AgentInstanceListResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-instances", query, nil, &result)
	return result, err
}
func (c *daemonClient) CreateAgentInstance(ctx context.Context, body contract.AgentInstanceCreateRequest) (contract.AgentInstanceResponse, error) {
	var result contract.AgentInstanceResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agent-instances", nil, body, &result)
	return result, err
}
func (c *daemonClient) AdoptAgentInstance(ctx context.Context, body contract.AgentInstanceAdoptRequest) (contract.AgentInstanceResponse, error) {
	var result contract.AgentInstanceResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agent-instances/adopt", nil, body, &result)
	return result, err
}
func (c *daemonClient) MutateAgentInstance(ctx context.Context, id, action string) (contract.AgentInstanceResponse, error) {
	var result contract.AgentInstanceResponse
	method, path := http.MethodPost, "/api/agent-instances/"+url.PathEscape(id)
	if action == "archive" {
		method = http.MethodDelete
	} else if action == "show" {
		method = http.MethodGet
	} else {
		path += "/" + action
	}
	err := c.doJSON(ctx, method, path, nil, nil, &result)
	return result, err
}
func (c *daemonClient) AgentInstanceReports(ctx context.Context, id string, query url.Values) (contract.AgentInstanceReportsResponse, error) {
	var result contract.AgentInstanceReportsResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-instances/"+url.PathEscape(id)+"/reports", query, nil, &result)
	return result, err
}
