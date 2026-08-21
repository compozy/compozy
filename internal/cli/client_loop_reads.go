package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
)

type loopRunReadClient interface {
	GetLoopRunNodes(context.Context, string, string, LoopRunNodesQuery) (contract.LoopRunNodesResponse, error)
	GetLoopRunBriefing(context.Context, string, string) (contract.LoopBriefingResponse, error)
	GetLoopRunTimeline(context.Context, string, string, LoopTimelineQuery) (contract.LoopTimelineResponse, error)
	StreamLoopRunEvents(context.Context, string, string, int64, SSEHandler) error
}

type LoopRunNodesQuery struct {
	State      string
	Generation int
	Cursor     string
	Limit      int
}

type LoopTimelineQuery struct {
	View   string
	Cursor string
	Limit  int
	After  int64
}

func (c *daemonClient) GetLoopRunNodes(
	ctx context.Context,
	workspaceID string,
	runID string,
	query LoopRunNodesQuery,
) (contract.LoopRunNodesResponse, error) {
	var response contract.LoopRunNodesResponse
	values := url.Values{}
	setLoopListQueryValue(values, "state", query.State)
	setLoopListQueryValue(values, "cursor", query.Cursor)
	if query.Generation > 0 {
		values.Set("generation", strconv.Itoa(query.Generation))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		loopRunPath(workspaceID, runID)+"/nodes",
		values,
		nil,
		&response,
	); err != nil {
		return contract.LoopRunNodesResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) GetLoopRunBriefing(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopBriefingResponse, error) {
	var response contract.LoopBriefingResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		loopRunPath(workspaceID, runID)+"/briefing",
		nil,
		nil,
		&response,
	); err != nil {
		return contract.LoopBriefingResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) GetLoopRunTimeline(
	ctx context.Context,
	workspaceID string,
	runID string,
	query LoopTimelineQuery,
) (contract.LoopTimelineResponse, error) {
	var response contract.LoopTimelineResponse
	values := url.Values{}
	setLoopListQueryValue(values, "view", query.View)
	setLoopListQueryValue(values, "cursor", query.Cursor)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.After > 0 {
		values.Set("after_sequence", strconv.FormatInt(query.After, 10))
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		loopRunPath(workspaceID, runID)+"/timeline",
		values,
		nil,
		&response,
	); err != nil {
		return contract.LoopTimelineResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) StreamLoopRunEvents(
	ctx context.Context,
	workspaceID string,
	runID string,
	after int64,
	handler SSEHandler,
) error {
	values := url.Values{}
	values.Set("after_sequence", strconv.FormatInt(after, 10))
	return c.doSSE(ctx, loopRunPath(workspaceID, runID)+"/events", values, strconv.FormatInt(after, 10), handler)
}
