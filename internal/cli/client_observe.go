package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// ObserveOverviewQuery bounds one home overview read.
type ObserveOverviewQuery struct {
	Workspace       string
	UsageWindowDays int
}

func (c *unixSocketClient) ObserveOverview(
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
