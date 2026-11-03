package cli

import (
	"context"
	"net/http"
)

func (c *daemonClient) MoveSessionToProfile(
	ctx context.Context,
	id string,
	request MoveSessionToProfileRequest,
) (MoveSessionToProfileResponse, error) {
	var response MoveSessionToProfileResponse
	path, err := c.sessionScopedPath(ctx, id, "/move-to-profile")
	if err != nil {
		return MoveSessionToProfileResponse{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return MoveSessionToProfileResponse{}, err
	}
	return response, nil
}
