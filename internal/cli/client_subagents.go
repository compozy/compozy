package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
)

type SubagentListQuery struct {
	Origin string
	Status string
	Limit  int
	Cursor string
}

type sessionSubagentsClient interface {
	ListSessionSubagents(context.Context, string, SubagentListQuery) (contract.SubagentListPayload, error)
	GetSubagent(context.Context, string, string) (contract.SubagentPayload, error)
	CancelSubagent(context.Context, string, string, string) (contract.SubagentCancelPayload, error)
}

var _ sessionSubagentsClient = (*daemonClient)(nil)

func (c *daemonClient) ListSessionSubagents(ctx context.Context, id string, query SubagentListQuery) (contract.SubagentListPayload, error) {
	var result contract.SubagentListPayload
	path, err := c.sessionScopedPath(ctx, id, "/subagents")
	if err != nil {
		return result, subagentClientError(err, "session_not_found", "Session "+id+" not found.")
	}
	values := url.Values{}
	if query.Origin != "" {
		values.Set("origin", query.Origin)
	}
	if query.Status != "" {
		values.Set("status", query.Status)
	}
	if query.Cursor != "" {
		values.Set("cursor", query.Cursor)
	}
	if query.Limit != 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	err = c.doJSON(ctx, http.MethodGet, path, values, nil, &result)
	return result, subagentClientError(err, "session_not_found", "Session "+id+" not found.")
}

func (c *daemonClient) GetSubagent(ctx context.Context, workspaceID, id string) (contract.SubagentPayload, error) {
	var result contract.SubagentPayload
	if workspaceID == "" {
		return c.findSubagent(ctx, id)
	}
	path, err := subagentClientPath(workspaceID, id)
	if err != nil {
		return result, err
	}
	err = c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, subagentClientError(err, "subagent_not_found", "subagent "+id+" not found")
}

func (c *daemonClient) CancelSubagent(ctx context.Context, workspaceID, id, reason string) (contract.SubagentCancelPayload, error) {
	var result contract.SubagentCancelPayload
	path, err := subagentClientPath(workspaceID, id)
	if err != nil {
		return result, err
	}
	err = c.doJSON(ctx, http.MethodPost, path+"/cancel", nil, contract.SubagentCancelRequest{Reason: reason}, &result)
	return result, subagentClientError(err, "subagent_not_found", "subagent "+id+" not found")
}

func subagentClientPath(workspaceID, id string) (string, error) {
	workspaceID, err := requirePathValue("workspace_id", workspaceID)
	if err != nil {
		return "", err
	}
	id, err = requirePathValue("subagent_id", id)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceID) + "/subagents/" + url.PathEscape(id), nil
}

func subagentClientError(err error, notFoundCode, notFoundMessage string) error {
	if err == nil {
		return nil
	}
	apiError, ok := errors.AsType[*daemonAPIError](err)
	if !ok {
		return err
	}
	if apiError.statusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %s", notFoundCode, notFoundMessage)
	}
	if apiError.payload.Code != "" {
		return fmt.Errorf("%s: %s", apiError.payload.Code, apiError.payload.Error)
	}
	return err
}

// findSubagent resolves a globally unique ID through authorized workspace routes.
func (c *daemonClient) findSubagent(ctx context.Context, id string) (contract.SubagentPayload, error) {
	workspaces, err := c.ListWorkspaces(ctx)
	if err != nil {
		return contract.SubagentPayload{}, err
	}
	for _, workspace := range workspaces {
		path, err := subagentClientPath(workspace.ID, id)
		if err != nil {
			return contract.SubagentPayload{}, err
		}
		var row contract.SubagentPayload
		err = c.doJSON(ctx, http.MethodGet, path, nil, nil, &row)
		if err == nil {
			return row, nil
		}
		apiError, ok := errors.AsType[*daemonAPIError](err)
		if !ok || apiError.statusCode != http.StatusNotFound {
			return row, err
		}
	}
	return contract.SubagentPayload{}, fmt.Errorf("subagent_not_found: subagent %s not found", id)
}
