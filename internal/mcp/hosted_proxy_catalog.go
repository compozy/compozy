package mcp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type hostedProxyCatalog struct {
	mu           sync.Mutex
	catalog      tools.AdvertisedCatalog
	threshold    int
	deferred     map[tools.ToolID]*tools.ToolView
	activated    map[tools.ToolID]struct{}
	awaitingList chan struct{}
}

func (c *hostedProxyCatalog) apply(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, projection HostedProjectionResponse,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	full := tools.NewAdvertisedCatalog(projection.Tools)
	if c.threshold <= 0 {
		applyHostedTools(server, client, bindID, projection.Tools)
	} else {
		c.applyBudgetedListing(server, client, bindID, projection.Tools, full.Digest)
	}
	c.catalog = full
}

func (c *hostedProxyCatalog) middleware(
	server *sdkmcp.Server, client HostedProxyClient, bindID string,
) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != "tools/list" {
				result, err := next(ctx, method, req)
				call, ok := req.(*sdkmcp.CallToolRequest)
				if c.threshold > 0 && err == nil && method == "tools/call" && ok && call.Params != nil {
					c.activateDiscovered(ctx, server, client, bindID, call, result)
				}
				return result, err
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}
			if err := client.ReportHostedMCPServed(
				ctx,
				HostedServedRequest{BindID: bindID, Digest: c.catalog.Digest},
			); err != nil {
				slog.WarnContext(ctx, "mcp.hosted.served_receipt_failed", "bind_id", bindID, "error", err)
			}
			if c.awaitingList != nil {
				close(c.awaitingList)
				c.awaitingList = nil
			}
			return result, nil
		}
	}
}

func (c *hostedProxyCatalog) activateDiscovered(
	ctx context.Context, server *sdkmcp.Server, client HostedProxyClient, bindID string,
	call *sdkmcp.CallToolRequest, result sdkmcp.Result,
) {
	ids := hostedDiscoveryIDs(tools.ToolID(call.Params.Name), result)
	if len(ids) == 0 {
		return
	}
	updated := c.activate(server, client, bindID, ids)
	if updated == nil {
		return
	}
	// The SDK debounces list_changed. Give the client time to refresh before it advances the turn.
	wait := time.NewTimer(250 * time.Millisecond)
	defer wait.Stop()
	select {
	case <-updated:
	case <-wait.C:
	case <-ctx.Done():
	}
}
