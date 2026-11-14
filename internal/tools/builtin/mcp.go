package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const mcpKey = "mcp"

var mcpTools = []toolspkg.Descriptor{
	nativeDescriptor(toolspkg.ToolIDMCPAllow, "mcp_allow", "Allow MCP Server", "Allow or revoke an exact project MCP definition.", mcpAllowInputSchema, toolspkg.RiskMutating, false, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDMCP}, []string{"mcp", "approval"}, []string{"allow mcp", "revoke mcp"}),
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

const mcpAllowInputSchema = `{"type":"object","required":["server_name","decision"],"properties":{"server_name":{"type":"string"},"workspace":{"type":"string"},"decision":{"type":"string","enum":["allow","revoke"]},"definition_digest":{"type":"string"}},"additionalProperties":false}`
