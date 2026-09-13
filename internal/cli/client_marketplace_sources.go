package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

type marketplaceSourcesReader interface {
	ListMarketplaceSources(context.Context) (contract.MarketplaceSourcesResponse, error)
}

type MarketplaceSourcesClient interface {
	marketplaceSourcesReader
	AddMarketplaceSource(
		context.Context,
		contract.AddMarketplaceSourceRequest,
	) (contract.MarketplaceSourceResponse, error)
	UpdateMarketplaceSource(
		context.Context,
		string,
		contract.UpdateMarketplaceSourceRequest,
	) (contract.MarketplaceSourceResponse, error)
	RemoveMarketplaceSource(context.Context, string) error
	RefreshMarketplaceSource(context.Context, string) (contract.MarketplaceSourceResponse, error)
}

func (c *daemonClient) ListMarketplaceSources(ctx context.Context) (contract.MarketplaceSourcesResponse, error) {
	var response contract.MarketplaceSourcesResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/marketplace/sources", nil, nil, &response)
	return response, err
}

func (c *daemonClient) AddMarketplaceSource(
	ctx context.Context,
	req contract.AddMarketplaceSourceRequest,
) (contract.MarketplaceSourceResponse, error) {
	var response contract.MarketplaceSourceResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/marketplace/sources", nil, req, &response)
	return response, err
}

func (c *daemonClient) RemoveMarketplaceSource(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/marketplace/sources/"+url.PathEscape(name), nil, nil, nil)
}

func (c *daemonClient) RefreshMarketplaceSource(
	ctx context.Context,
	name string,
) (contract.MarketplaceSourceResponse, error) {
	var response contract.MarketplaceSourceResponse
	err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/marketplace/sources/"+url.PathEscape(name)+"/refresh",
		nil,
		nil,
		&response,
	)
	return response, err
}

func (c *daemonClient) UpdateMarketplaceSource(
	ctx context.Context, name string, req contract.UpdateMarketplaceSourceRequest,
) (contract.MarketplaceSourceResponse, error) {
	var response contract.MarketplaceSourceResponse
	err := c.doJSON(ctx, http.MethodPatch, "/api/marketplace/sources/"+url.PathEscape(name), nil, req, &response)
	return response, err
}
