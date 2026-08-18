package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

type LoopRequestListQuery struct {
	RunID  string
	State  string
	Cursor string
	Limit  int
}

func (c *daemonClient) ListLoopRequests(
	ctx context.Context,
	workspaceID string,
	query LoopRequestListQuery,
) (contract.LoopRequestsResponse, error) {
	var response contract.LoopRequestsResponse
	path := loopWorkspacePath(workspaceID) + "/loop-requests"
	values := url.Values{}
	setLoopListQueryValue(values, "run_id", query.RunID)
	setLoopListQueryValue(values, "state", query.State)
	setLoopListQueryValue(values, "cursor", query.Cursor)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return contract.LoopRequestsResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) GetLoopRequest(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int,
	nodeID string,
	itemIndex int,
) (contract.LoopRequestPayload, error) {
	var response contract.LoopRequestPayload
	path := loopRequestPath(workspaceID, runID, nodeID)
	values := url.Values{
		loopGenerationKey: []string{strconv.Itoa(generation)},
		"item_index":      []string{strconv.Itoa(itemIndex)},
	}
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return contract.LoopRequestPayload{}, err
	}
	return response, nil
}

func (c *daemonClient) RespondLoopRequest(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.RespondLoopRequest,
	credentials agentidentity.Credentials,
) (contract.RespondLoopRequestResponse, error) {
	var response contract.RespondLoopRequestResponse
	path := loopRunPath(workspaceID, runID) + "/nodes/" + url.PathEscape(strings.TrimSpace(nodeID)) + "/respond"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return contract.RespondLoopRequestResponse{}, err
	}
	return response, nil
}

func loopRequestPath(workspaceID string, runID string, nodeID string) string {
	return loopRunPath(workspaceID, runID) + "/nodes/" + url.PathEscape(strings.TrimSpace(nodeID)) + "/request"
}
