package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) ListSessionProcesses(ctx context.Context, id string) (contract.SessionProcessesResponse, error) {
	result := contract.SessionProcessesResponse{}
	path, err := c.sessionScopedPath(ctx, id, "/processes")
	if err != nil {
		return result, err
	}
	err = c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, err
}

func (c *daemonClient) TerminateSessionProcesses(
	ctx context.Context,
	id string,
	request contract.TerminateSessionProcessesRequest,
) (contract.TerminateSessionProcessesResponse, error) {
	result := contract.TerminateSessionProcessesResponse{}
	path, err := c.sessionScopedPath(ctx, id, "/processes/terminate")
	if err != nil {
		return result, err
	}
	err = c.doJSONWithClient(ctx, http.MethodPost, path, nil, request, &result, c.streamHTTPClient())
	return result, err
}

func (c *daemonClient) ListProcesses(
	ctx context.Context,
	workspaceID string,
	allWorkspaces, orphaned bool,
) (contract.ProcessesResponse, error) {
	result := contract.ProcessesResponse{}
	query := url.Values{
		"all_workspaces": {strconv.FormatBool(allWorkspaces)},
		"orphaned":       {strconv.FormatBool(orphaned)},
	}
	if workspaceID != "" {
		query.Set("workspace_id", workspaceID)
	} else {
		query.Set("all_workspaces", "true")
	}
	err := c.doJSON(ctx, http.MethodGet, "/api/processes", query, nil, &result)
	return result, err
}
