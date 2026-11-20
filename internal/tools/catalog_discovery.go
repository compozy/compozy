package tools

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"strings"
)

// CatalogDiscoveryLimit is the maximum number of IDs in one native discovery page.
const CatalogDiscoveryLimit = 100

// IsCatalogDiscoveryTool identifies the native catalog results that can activate hosted MCP tools.
func IsCatalogDiscoveryTool(id ToolID) bool {
	return id == ToolIDToolList || id == ToolIDToolSearch || id == ToolIDToolInfo
}

type catalogDiscoveryKey struct{}

// WithCatalogDiscovery retains authorized MCP IDs beside the public, potentially offloaded result.
func WithCatalogDiscovery(ctx context.Context) (context.Context, func() []ToolID) {
	var ids []ToolID
	return context.WithValue(ctx, catalogDiscoveryKey{}, &ids), func() []ToolID { return ids }
}

func captureCatalogDiscovery(ctx context.Context, d Descriptor, result ToolResult) {
	ids, ok := ctx.Value(catalogDiscoveryKey{}).(*[]ToolID)
	if !ok || d.Source.Kind != SourceBuiltin || !IsCatalogDiscoveryTool(d.ID) ||
		bytes.Equal(result.Metadata["is_error"], []byte("true")) {
		return
	}
	type row struct {
		Descriptor struct {
			ID string `json:"id"`
		} `json:"descriptor"`
		Decision struct {
			Callable bool `json:"callable"`
		} `json:"decision"`
	}
	var payload struct {
		Tool  row   `json:"tool"`
		Tools []row `json:"tools"`
	}
	if len(result.Structured) == 0 || json.Unmarshal(result.Structured, &payload) != nil {
		return
	}
	rows := payload.Tools[:min(len(payload.Tools), CatalogDiscoveryLimit)]
	if d.ID == ToolIDToolInfo {
		rows = []row{payload.Tool}
	}
	for _, entry := range rows {
		id := ToolID(entry.Descriptor.ID)
		if entry.Decision.Callable && strings.HasPrefix(id.String(), "mcp__") && id.Validate() == nil {
			*ids = append(*ids, id)
		}
	}
}
