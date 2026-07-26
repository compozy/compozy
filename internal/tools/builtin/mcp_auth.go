package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	mcpAuthStatusKey = "status"
)

var mcpAuthTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMCPAuthStatus,
		"mcp_auth_status",
		"MCP Auth Status",
		"Read redacted MCP auth diagnostics for one configured server.",
		mcpAuthStatusInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMCPAuth},
		[]string{"mcp", "auth", mcpAuthStatusKey},
		[]string{"mcp auth status", "oauth status", "mcp login diagnostics"},
	),
}

func mcpAuthDescriptors() []toolspkg.Descriptor {
	return mcpAuthTools
}

const mcpAuthStatusInputSchema = `{
	"type":"object",
	"required":["server_name"],
	"properties":{
		"server_name":{"type":"string"}
	},
	"additionalProperties":false
}`
