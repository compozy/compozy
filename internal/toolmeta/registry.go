package toolmeta

import "strings"

// NativeEntry returns curated progress metadata for Compozy's native tool namespace.
func NativeEntry(toolID string) (Entry, bool) {
	entry, ok := nativeEntries[NormalizeHostedToolName(toolID)]
	return entry, ok
}

// NormalizeHostedToolName removes provider wrappers for the Compozy hosted MCP server.
func NormalizeHostedToolName(name string) string {
	name = strings.TrimSpace(name)
	for _, prefix := range []string{"mcp__compozy-hosted-tools__", "mcp.compozy-hosted-tools.", "compozy-hosted-tools."} {
		if trimmed, ok := strings.CutPrefix(name, prefix); ok {
			return trimmed
		}
	}
	return name
}
