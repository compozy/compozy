package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) AmendLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeAmendRequest,
	credentials agentidentity.Credentials,
) (contract.LoopNodeAmendResponse, error) {
	var response contract.LoopNodeAmendResponse
	path := loopRunPath(workspaceID, runID) + "/nodes/" + url.PathEscape(strings.TrimSpace(nodeID)) + "/amend"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return contract.LoopNodeAmendResponse{}, err
	}
	return response, nil
}
