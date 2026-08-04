package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const mcpKey = "mcp"

var mcpTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMCPStatus,
		"mcp_status",
		"MCP Status",
		"Probe one configured MCP server without exposing login or logout as tool calls.",
		mcpAuthStatusInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, descriptorKeywordStatus, "probe"},
		[]string{"mcp status", "mcp probe", "mcp server health"},
	),
}

func mcpDescriptors() []toolspkg.Descriptor {
	return mcpTools
}
