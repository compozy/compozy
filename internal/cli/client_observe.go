package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

// ObserveOverviewQuery bounds one home overview read.
type ObserveOverviewQuery struct {
	Workspace       string
	UsageWindowDays int
}

func (c *daemonClient) ObserveOverview(
	ctx context.Context,
	query ObserveOverviewQuery,
) (contract.ObserveOverviewResponse, error) {
	var response contract.ObserveOverviewResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/observe/overview",
		observeOverviewQueryValues(query),
		nil,
		&response,
	); err != nil {
		return contract.ObserveOverviewResponse{}, err
	}
	return response, nil
}

func observeOverviewQueryValues(query ObserveOverviewQuery) url.Values {
	values := url.Values{}
	if workspace := strings.TrimSpace(query.Workspace); workspace != "" {
		values.Set("workspace", workspace)
	}
	if query.UsageWindowDays > 0 {
		values.Set("usage_window", strconv.Itoa(query.UsageWindowDays))
	}
	return values
}

type PermissionRollupQuery struct {
	Days       int
	Agent      string
	Unattended bool
}

func (c *daemonClient) PermissionRollup(ctx context.Context, query PermissionRollupQuery) (contract.PermissionRollupResponse, error) {
	values := url.Values{}
	values.Set("days", strconv.Itoa(query.Days))
	values.Set("unattended", strconv.FormatBool(query.Unattended))
	if agent := strings.TrimSpace(query.Agent); agent != "" {
		values.Set("agent", agent)
	}
	var response contract.PermissionRollupResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/permissions/rollup", values, nil, &response); err != nil {
		return contract.PermissionRollupResponse{}, err
	}
	return response, nil
}
