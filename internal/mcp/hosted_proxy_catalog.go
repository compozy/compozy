package mcp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const hostedDiscoveryRefreshTimeout = 250 * time.Millisecond

type hostedProxyCatalog struct {
	mu        sync.Mutex
	catalog   tools.AdvertisedCatalog
	threshold int
	search    *tools.Descriptor
	listed    map[string]string
	listedAt  map[string]uint64
	deferred  map[tools.ToolID]*tools.ToolView
	directory map[string]int
	pending   map[tools.ToolID]time.Time
	changed   chan struct{}
	delivery  hostedListDelivery
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

func (c *hostedProxyCatalog) middleware(client HostedProxyClient, bindID string) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != mcpToolsListMethod {
				return next(ctx, method, req)
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
			if c.threshold > 0 {
				c.delivery.record(req, result)
			}
			return result, nil
		}
	}
}

func (c *hostedProxyCatalog) signal() {
	if c.changed != nil {
		close(c.changed)
	}
	c.changed = make(chan struct{})
}

func (c *hostedProxyCatalog) unready(ids []tools.ToolID) (int, <-chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, id := range ids {
		view := c.deferred[id]
		if !c.nameDelivered(id.String()) && (view == nil || hostedToolActivatable(view)) {
			count++
		}
	}
	return count, c.changed
}

func (c *hostedProxyCatalog) nameDelivered(name string) bool {
	revision, listed := c.listedAt[name]
	return listed && revision <= c.delivery.delivered
}

func (c *hostedProxyCatalog) awaitDiscovery(ctx context.Context, ids []tools.ToolID) int {
	delay := time.NewTimer(hostedDiscoveryRefreshTimeout)
	defer delay.Stop()
	for {
		count, changed := c.unready(ids)
		if count == 0 {
			return 0
		}
		select {
		case <-changed:
		case <-delay.C:
			count, _ = c.unready(ids)
			return count
		case <-ctx.Done():
			return count
		}
	}
}
