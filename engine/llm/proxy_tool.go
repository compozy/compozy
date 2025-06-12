package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/mcp"
	"github.com/tmc/langchaingo/tools"
)

// ProxyTool implements a langchain tool that executes via the MCP proxy
type ProxyTool struct {
	name        string
	description string
	inputSchema map[string]any
	mcpName     string
	proxyClient *mcp.Client
}

// NewProxyTool creates a new proxy tool from a tool definition
func NewProxyTool(toolDef mcp.ToolDefinition, proxyClient *mcp.Client) tools.Tool {
	return &ProxyTool{
		name:        toolDef.Name,
		description: toolDef.Description,
		inputSchema: toolDef.InputSchema,
		mcpName:     toolDef.MCPName,
		proxyClient: proxyClient,
	}
}

// Name returns the tool name
func (t *ProxyTool) Name() string {
	return t.name
}

// Description returns the tool description
func (t *ProxyTool) Description() string {
	return t.description
}

// Call executes the tool via the proxy
func (t *ProxyTool) Call(_ context.Context, input string) (string, error) {
	// Parse the input arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Execute the tool via proxy (this would need to be implemented in the proxy)
	// For now, we return a placeholder indicating proxy execution
	result := fmt.Sprintf("Executed tool '%s' from MCP '%s' via proxy with args: %s",
		t.name, t.mcpName, input)

	return result, nil
}

// ArgsType returns the input schema (not implemented for langchain tools)
func (t *ProxyTool) ArgsType() any {
	return t.inputSchema
}
