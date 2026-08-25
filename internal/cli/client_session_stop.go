package cli

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) StopSessionSubtree(
	ctx context.Context,
	id string,
	reason string,
) (contract.StopSessionSubtreeResponse, error) {
	path, err := c.sessionScopedPath(ctx, id, "/stop")
	if err != nil {
		return contract.StopSessionSubtreeResponse{}, err
	}
	var response contract.StopSessionSubtreeResponse
	err = c.doJSON(ctx, http.MethodPost, path, nil, contract.StopSessionRequest{
		Subtree: true, Reason: strings.TrimSpace(reason),
	}, &response)
	return response, err
}
