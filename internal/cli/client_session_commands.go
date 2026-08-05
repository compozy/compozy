package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

// ListSessionCommands returns the unified command catalog for one session.
func (c *unixSocketClient) ListSessionCommands(
	ctx context.Context,
	id string,
) (SessionCommandsRecord, error) {
	path, err := c.sessionScopedPath(ctx, id, "/commands")
	if err != nil {
		return SessionCommandsRecord{}, err
	}
	var response contract.SessionCommandsResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionCommandsRecord{}, err
	}
	return response, nil
}
