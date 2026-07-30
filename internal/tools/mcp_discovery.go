package tools

// DeferInitialDiscovery keeps vendor MCP startup out of the hosted session handshake.
func (*MCPProvider) DeferInitialDiscovery() bool {
	return true
}
