package mcp

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type hostedProxyCatalog struct {
	mu      sync.RWMutex
	catalog tools.AdvertisedCatalog
}

func (c *hostedProxyCatalog) apply(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, projection HostedProjectionResponse,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	applyHostedTools(server, client, bindID, projection.Tools)
	c.catalog = projection.Catalog
}

func (c *hostedProxyCatalog) middleware(client HostedProxyClient, bindID string) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != "tools/list" {
				return next(ctx, method, req)
			}
			c.mu.RLock()
			defer c.mu.RUnlock()
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}
			if err := client.ReportHostedMCPServed(ctx, HostedServedRequest{BindID: bindID, Digest: c.catalog.Digest}); err != nil {
				return nil, err
			}
			return result, nil
		}
	}
}
