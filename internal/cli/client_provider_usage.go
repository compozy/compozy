package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

type ProviderUsageClient interface {
	ListProviderUsage(context.Context, string) (contract.ProviderUsageEnvelope, error)
	RefreshProviderUsage(context.Context, contract.ProviderUsageRefreshRequest) (contract.ProviderUsageEnvelope, error)
}

func (c *daemonClient) ListProviderUsage(ctx context.Context, provider string) (contract.ProviderUsageEnvelope, error) {
	path := "/api/provider-usage"
	if provider != "" {
		path += "/providers/" + url.PathEscape(provider)
	}
	var result contract.ProviderUsageEnvelope
	err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, err
}

func (c *daemonClient) RefreshProviderUsage(ctx context.Context, request contract.ProviderUsageRefreshRequest) (contract.ProviderUsageEnvelope, error) {
	if request.Profile == "" {
		request.Profile = profileQueryValues(ctx, nil).Get("profile")
	}
	var result contract.ProviderUsageEnvelope
	err := c.doJSON(ctx, http.MethodPost, "/api/provider-usage/refresh", nil, request, &result)
	return result, err
}
