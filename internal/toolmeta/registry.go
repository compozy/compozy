package toolmeta

import "strings"

// HostedServerName is the MCP server name Compozy injects into every ACP
// session for its hosted native tools.
const HostedServerName = "compozy-hosted-tools"

// HostedBindNonceFlag marks the daemon-minted launch of the hosted MCP server;
// a configured server that only reuses the name never carries it.
const HostedBindNonceFlag = "--bind-nonce"

// NativeEntry returns curated progress metadata for Compozy's native tool namespace.
func NativeEntry(toolID string) (Entry, bool) {
	entry, ok := nativeEntries[NormalizeHostedToolName(toolID)]
	return entry, ok
}

// NormalizeHostedToolName removes provider wrappers for the Compozy hosted MCP server.
func NormalizeHostedToolName(name string) string {
	name = strings.TrimSpace(name)
	prefixes := []string{"mcp__" + HostedServerName + "__", "mcp." + HostedServerName + ".", HostedServerName + "."}
	for _, prefix := range prefixes {
		if trimmed, ok := strings.CutPrefix(name, prefix); ok {
			return trimmed
		}
	}
	return name
}
