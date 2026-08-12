package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) RenameSession(
	ctx context.Context,
	id string,
	request RenameSessionRequest,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}
