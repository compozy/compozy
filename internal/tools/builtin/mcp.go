package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const mcpKey = "mcp"

var mcpTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDSessionMCPList,
		"session_mcp_list",
		"Session MCP Servers",
		"Read the session MCP servers, restrictions and pending changes without discovery. Defaults to the caller session.",
		`{"type":"object","properties":{"session_id":{"type":"string"}},"additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, "session"},
		[]string{"session mcp servers"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMCPImport,
		"mcp_import",
		"Copy MCP Server",
		"Copy one representable runtime MCP server into CompozyOS without modifying its source. "+
			"Literal credentials move into Vault; OAuth requires a new sign-in.",
		mcpImportInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, "copy", "import"},
		[]string{"copy runtime mcp", "import mcp"},
	),
	nativeDescriptor(toolspkg.ToolIDMCPList, "mcp_list", "MCP Servers",
		"List CompozyOS and runtime MCP server definitions without starting or probing them.",
		mcpListInputSchema, toolspkg.RiskRead, true, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, descriptorKeywordList, "runtime"}, []string{"list mcp servers", "runtime mcp inventory"}),
	nativeDescriptor(
		toolspkg.ToolIDMCPAllow,
		"mcp_allow",
		"Allow MCP Server",
		"Allow or revoke an exact project MCP definition.",
		mcpAllowInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCP},
		[]string{mcpKey, "approval"},
		[]string{"allow mcp", "revoke mcp"},
	),
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

const mcpAllowInputSchema = `{"type":"object","required":["server_name","decision"],"properties":{"server_name":{"type":"string"},"workspace":{"type":"string"},"decision":{"type":"string","enum":["allow","revoke"]},"definition_digest":{"type":"string"}},"additionalProperties":false}`

const mcpImportInputSchema = `{"type":"object","required":["runtime","server_name"],"properties":{"runtime":{"type":"string","minLength":1},"server_name":{"type":"string","minLength":1},"scope":{"type":"string","enum":["user","profile","workspace"],"default":"user"},"workspace":{"type":"string"},"name":{"type":"string"}},"additionalProperties":false}`
