package mcp

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	hostedDirectoryMaxServers   = 16
	hostedDirectoryMaxNameBytes = 72
	hostedPendingDiscoveryLimit = 512
	hostedPendingDiscoveryTTL   = 30 * time.Second
)

type hostedDiscovery func(context.Context, []tools.ToolID) int

type hostedListing struct {
	views     []tools.ToolView
	directory string
	discover  hostedDiscovery
}

func (c *hostedProxyCatalog) applyBudgetedListing(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, views []tools.ToolView, digest string,
) {
	c.prunePending(time.Now())
	previous := c.listed
	counts := make(map[string]int)
	c.search = nil
	for i := range views {
		d := &views[i].Descriptor
		if d.Source.Kind == tools.SourceMCP {
			counts[d.Source.RawServerName]++
		}
		if d.ID == tools.ToolIDToolSearch {
			c.search = d
		}
	}
	deferred := make(map[tools.ToolID]*tools.ToolView)
	servers := make(map[string]int)
	listing := hostedListing{
		views: make([]tools.ToolView, 0, len(views)),
		discover: func(ctx context.Context, ids []tools.ToolID) int {
			return c.discover(ctx, server, client, bindID, ids)
		},
	}
	for i := range views {
		view := &views[i]
		d := &view.Descriptor
		name := d.Source.RawServerName
		count := counts[name]
		_, listed := previous[d.ID.String()]
		_, requested := c.pending[d.ID]
		promoted := requested && view.Decision.Callable
		delete(c.pending, d.ID)
		if c.search != nil && d.Source.Kind == tools.SourceMCP && count > c.threshold &&
			!listed && !promoted {
			deferred[d.ID] = view
			servers[name] = count
			continue
		}
		listing.views = append(listing.views, *view)
	}
	listing.directory = hostedServerDirectory(servers)
	if c.search != nil && c.catalog.Digest != "" && digest != c.catalog.Digest &&
		(len(c.deferred) > 0 || len(deferred) > 0) {
		// A change to hidden tools can leave the visible list identical but still needs a new receipt.
		delete(previous, tools.ToolIDToolSearch.String())
	}
	next := reconcileHostedListing(server, client, bindID, previous, listing)
	if !maps.Equal(previous, next) {
		c.delivery.revision++
		c.signal()
	}
	c.listed, c.deferred = next, deferred
}

func hostedServerDirectory(servers map[string]int) string {
	if len(servers) == 0 {
		return ""
	}
	var directory strings.Builder
	directory.WriteString(
		"\n\nMCP servers with deferred tools. Search by server name and action to register callable matches.",
	)
	names := slices.Sorted(maps.Keys(servers))
	for _, name := range names[:min(len(names), hostedDirectoryMaxServers)] {
		label := diagnostics.RedactAndBound(strings.Join(strings.Fields(name), " "), hostedDirectoryMaxNameBytes)
		fmt.Fprintf(&directory, "\n- %q (%d tools). External MCP server.", label, servers[name])
	}
	if more := len(servers) - hostedDirectoryMaxServers; more > 0 {
		fmt.Fprintf(&directory, "\n- %d more servers. Search across all servers.", more)
	}
	return directory.String()
}

func (c *hostedProxyCatalog) prunePending(now time.Time) {
	maps.DeleteFunc(c.pending, func(_ tools.ToolID, expires time.Time) bool { return !expires.After(now) })
}

func (c *hostedProxyCatalog) discover(
	ctx context.Context, server *sdkmcp.Server, client HostedProxyClient, bindID string, ids []tools.ToolID,
) int {
	if len(ids) == 0 {
		return 0
	}
	wait := c.activate(server, client, bindID, ids)
	if len(wait) == 0 {
		return 0
	}
	return c.awaitDiscovery(ctx, wait)
}

func (c *hostedProxyCatalog) activate(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, ids []tools.ToolID,
) []tools.ToolID {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.search == nil {
		return nil
	}
	now := time.Now()
	c.prunePending(now)
	wait := make([]tools.ToolID, 0, min(len(ids), tools.CatalogDiscoveryLimit))
	added := false
	for _, id := range ids[:min(len(ids), tools.CatalogDiscoveryLimit)] {
		if !strings.HasPrefix(id.String(), "mcp__") || slices.Contains(wait, id) {
			continue
		}
		if _, listed := c.listed[id.String()]; listed && c.delivery.delivered == c.delivery.revision {
			continue
		}
		if view, deferred := c.deferred[id]; deferred && view.Decision.Callable {
			tool := hostedMCPTool(view.Descriptor)
			c.listed[tool.Name] = registerHostedTool(server, client, bindID, view.Descriptor, &tool, "", nil)
			delete(c.deferred, id)
			added = true
		} else if _, listed := c.listed[id.String()]; !listed && !deferred {
			c.queuePending(id, now)
		}
		wait = append(wait, id)
	}
	if added {
		if len(c.deferred) == 0 {
			search := hostedMCPTool(*c.search)
			c.listed[search.Name] = registerHostedTool(
				server, client, bindID, *c.search, &search, c.listed[search.Name],
				func(ctx context.Context, ids []tools.ToolID) int {
					return c.discover(ctx, server, client, bindID, ids)
				},
			)
		}
		c.delivery.revision++
		c.signal()
	}
	return wait
}

func (c *hostedProxyCatalog) queuePending(id tools.ToolID, now time.Time) {
	if _, queued := c.pending[id]; queued || len(c.pending) >= hostedPendingDiscoveryLimit {
		return
	}
	if c.pending == nil {
		c.pending = make(map[tools.ToolID]time.Time)
	}
	c.pending[id] = now.Add(hostedPendingDiscoveryTTL)
}
