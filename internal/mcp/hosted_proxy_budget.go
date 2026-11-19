package mcp

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	hostedDirectoryMaxServers   = 16
	hostedDirectoryMaxNameBytes = 72
)

type hostedListing struct {
	views     []tools.ToolView
	activated map[tools.ToolID]struct{}
	directory string
}

func (c *hostedProxyCatalog) applyBudgetedListing(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, views []tools.ToolView, digest string,
) {
	previous := hostedToolFingerprints(server)
	counts := make(map[string]int)
	hasSearch := false
	for i := range views {
		d := &views[i].Descriptor
		if d.Source.Kind == tools.SourceMCP {
			counts[d.Source.RawServerName]++
		}
		hasSearch = hasSearch || d.ID == tools.ToolIDToolSearch
	}
	deferred := make(map[tools.ToolID]*tools.ToolView)
	servers := make(map[string]int)
	listing := hostedListing{
		views:     make([]tools.ToolView, 0, len(views)),
		activated: make(map[tools.ToolID]struct{}, len(c.activated)),
	}
	for i := range views {
		view := &views[i]
		d := &view.Descriptor
		name := d.Source.RawServerName
		count := counts[name]
		_, listed := previous[d.ID.String()]
		if hasSearch && d.Source.Kind == tools.SourceMCP && count > c.threshold && !listed {
			deferred[d.ID] = view
			servers[name] = count
			continue
		}
		if _, activated := c.activated[d.ID]; activated {
			listing.activated[d.ID] = struct{}{}
		}
		listing.views = append(listing.views, *view)
	}
	listing.directory = hostedServerDirectory(servers)
	if hasSearch && c.catalog.Digest != "" && digest != c.catalog.Digest &&
		(len(c.deferred) > 0 || len(deferred) > 0) {
		// A change to hidden tools can leave the visible list identical but still needs a new receipt.
		delete(previous, tools.ToolIDToolSearch.String())
	}
	setHostedToolFingerprints(server, reconcileHostedListing(server, client, bindID, previous, listing))
	c.deferred, c.activated = deferred, listing.activated
}

func hostedServerDirectory(servers map[string]int) string {
	if len(servers) == 0 {
		return ""
	}
	var directory strings.Builder
	directory.WriteString(
		"\n\nMCP servers with deferred tools. Search by server name and action to load callable matches.",
	)
	names := slices.Sorted(maps.Keys(servers))
	for _, name := range names[:min(len(names), hostedDirectoryMaxServers)] {
		label := diagnostics.RedactAndBound(strings.Join(strings.Fields(name), " "), hostedDirectoryMaxNameBytes)
		fmt.Fprintf(&directory, "\n- %q (%d tools): external MCP server.", label, servers[name])
	}
	if more := len(servers) - hostedDirectoryMaxServers; more > 0 {
		fmt.Fprintf(&directory, "\n- %d more servers. Search across all servers.", more)
	}
	return directory.String()
}

func (c *hostedProxyCatalog) activate(
	server *sdkmcp.Server, client HostedProxyClient, bindID string, ids []tools.ToolID,
) <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	var fingerprints map[string]string
	for _, id := range ids {
		view, ok := c.deferred[id]
		if !ok || !view.Decision.Callable {
			continue
		}
		if fingerprints == nil {
			fingerprints = hostedToolFingerprints(server)
		}
		tool := hostedMCPTool(view.Descriptor)
		tool.Meta["anthropic/alwaysLoad"] = true
		fingerprints[tool.Name] = registerHostedTool(server, client, bindID, view.Descriptor, &tool, "")
		delete(c.deferred, id)
		c.activated[id] = struct{}{}
	}
	if fingerprints == nil {
		return nil
	}
	setHostedToolFingerprints(server, fingerprints)
	if c.awaitingList == nil {
		c.awaitingList = make(chan struct{})
	}
	return c.awaitingList
}

func hostedDiscoveryIDs(discovery tools.ToolID, result sdkmcp.Result) []tools.ToolID {
	call, ok := result.(*sdkmcp.CallToolResult)
	if !ok || call.IsError {
		return nil
	}
	body, ok := call.StructuredContent.(map[string]any)
	if !ok {
		return nil
	}
	var ids []tools.ToolID
	switch discovery {
	case tools.ToolIDToolInfo:
		if id := hostedCallableDiscoveryID(body["tool"]); id != "" {
			ids = append(ids, id)
		}
	case tools.ToolIDToolSearch:
		rows, ok := body["tools"].([]any)
		if !ok {
			return nil
		}
		for _, row := range rows {
			if id := hostedCallableDiscoveryID(row); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func hostedCallableDiscoveryID(value any) tools.ToolID {
	row, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	decision, ok := row["decision"].(map[string]any)
	if !ok || decision["callable"] != true {
		return ""
	}
	descriptor, ok := row["descriptor"].(map[string]any)
	if !ok {
		return ""
	}
	id, ok := descriptor["id"].(string)
	if !ok {
		return ""
	}
	return tools.ToolID(id)
}
