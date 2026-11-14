package tools

import "fmt"

// MCPAuthRequiredError gives callers a stable repair action for signed-out servers.
func MCPAuthRequiredError(id ToolID, name string) error {
	return NewToolError("mcp_auth_required", id, fmt.Sprintf("%s needs sign-in. Sign in to %s in CompozyOS (Settings → MCP servers).", name, name), ErrToolUnavailable, ReasonMCPAuthRequired)
}
