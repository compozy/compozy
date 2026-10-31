package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

// MoveSessionToLater stops a session tree and parks it in Later.
func (c *daemonClient) MoveSessionToLater(
	ctx context.Context,
	id string,
	req contract.MoveSessionToLaterRequest,
) (SessionRecord, error) {
	return c.postSessionLater(ctx, id, "/later", req)
}

// ResumeSessionFromLater returns a parked session to the normal flow.
func (c *daemonClient) ResumeSessionFromLater(
	ctx context.Context,
	id string,
	withChildren bool,
) (SessionRecord, error) {
	request := contract.ResumeSessionFromLaterRequest{WithChildren: withChildren}
	return c.postSessionLater(ctx, id, "/resume-later", request)
}

func (c *daemonClient) postSessionLater(
	ctx context.Context,
	id string,
	suffix string,
	body any,
) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, suffix)
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSONWithClient(ctx, http.MethodPost, path, nil, body, &response, c.streamHTTPClient()); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}
