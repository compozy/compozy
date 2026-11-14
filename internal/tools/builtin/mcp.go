package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const mcpKey = "mcp"

var mcpTools = []toolspkg.Descriptor{
	nativeDescriptor(toolspkg.ToolIDMCPList, "mcp_list", "MCP Servers",
		"List CompozyOS and runtime MCP server definitions without starting or probing them.",
		mcpListInputSchema, toolspkg.RiskRead, true, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, "list", "runtime"}, []string{"list mcp servers", "runtime mcp inventory"}),
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

const mcpListInputSchema = `{"type":"object","properties":{"runtime":{"type":"string"},"include_runtimes":{"type":"boolean","default":true},"workspace":{"type":"string"}},"additionalProperties":false}`
