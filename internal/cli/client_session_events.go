package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) SessionEvents(
	ctx context.Context,
	id string,
	query SessionEventQuery,
) ([]SessionEventRecord, error) {
	var response struct {
		Events []SessionEventRecord `json:"events"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/events")
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		sessionEventValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Events, nil
}

func (c *daemonClient) StreamSessionEvents(
	ctx context.Context,
	id string,
	query SessionEventQuery,
	lastEventID string,
	handler SSEHandler,
) error {
	path, err := c.sessionScopedPath(ctx, id, "/stream")
	if err != nil {
		return err
	}
	if query.Last == 0 {
		query.Last = 200
	}
	values := sessionEventValues(query)
	values.Set("frames", contract.SessionStreamFrameRaw)
	return c.doSSE(
		ctx,
		path,
		values,
		lastEventID,
		handler,
	)
}

func (c *daemonClient) SessionHistory(
	ctx context.Context,
	id string,
	query SessionEventQuery,
) ([]TurnHistoryRecord, error) {
	var response struct {
		History []TurnHistoryRecord `json:"history"`
	}
	path, err := c.sessionScopedPath(ctx, id, "/history")
	if err != nil {
		return nil, err
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		path,
		sessionEventValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.History, nil
}
