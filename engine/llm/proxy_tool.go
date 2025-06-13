package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
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
func (t *ProxyTool) Call(ctx context.Context, input string) (string, error) {
	// Parse the input arguments
	if input == "" {
		input = "{}"
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Validate arguments against input schema
	if err := t.validateArguments(args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	// Execute the tool via proxy
	result, err := t.proxyClient.CallTool(ctx, t.mcpName, t.name, args)
	if err != nil {
		return "", fmt.Errorf("failed to execute tool '%s' from MCP '%s': %w", t.name, t.mcpName, err)
	}

	// Preserve raw strings; JSON-encode structured payloads
	switch v := result.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v), nil
		}
		return string(b), nil
	}
}

// ArgsType returns the input schema (not implemented for langchain tools)
func (t *ProxyTool) ArgsType() any {
	return t.inputSchema
}

// validateArguments validates the provided arguments against the tool's input schema
func (t *ProxyTool) validateArguments(args map[string]any) error {
	// Skip validation if no schema is defined
	if len(t.inputSchema) == 0 {
		return nil
	}

	// Convert map[string]any to schema.Schema and use the engine/schema package
	toolSchema := schema.Schema(t.inputSchema)
	validator := schema.NewParamsValidator(args, &toolSchema, fmt.Sprintf("tool:%s", t.name))

	return validator.Validate(context.Background())
}
