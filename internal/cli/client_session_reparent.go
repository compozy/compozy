package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) SetSessionParent(
	ctx context.Context,
	id string,
	request SetSessionParentRequest,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/parent")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}

func (c *daemonClient) ClearSessionParent(ctx context.Context, id string) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/parent")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}
