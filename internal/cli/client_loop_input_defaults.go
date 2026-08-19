package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) GetLoopInputDefaults(
	ctx context.Context,
	workspaceID string,
	name string,
	scope contract.LoopInputDefaultsScope,
) (contract.LoopInputDefaultsResponse, error) {
	var response contract.LoopInputDefaultsResponse
	query := url.Values{automationScopeKey: []string{string(scope)}}
	path := loopDefinitionPath(workspaceID, name) + "/input-defaults"
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return contract.LoopInputDefaultsResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) PutLoopInputDefault(
	ctx context.Context,
	workspaceID string,
	name string,
	key string,
	request contract.PutLoopInputDefaultRequest,
) (contract.LoopInputDefaultResponse, error) {
	var response contract.LoopInputDefaultResponse
	path := loopDefinitionPath(workspaceID, name) + "/input-defaults/" + url.PathEscape(key)
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return contract.LoopInputDefaultResponse{}, err
	}
	return response, nil
}
