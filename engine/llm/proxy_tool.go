package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	mcptools "github.com/compozy/compozy/engine/mcp/tools"
	"github.com/compozy/compozy/engine/schema"
	"github.com/tmc/langchaingo/tools"
)

// mcpExecutor is the minimal contract needed by ProxyTool.
type mcpExecutor interface {
	Execute(ctx context.Context, mcpName, toolName string, args map[string]any) (any, error)
}

// ProxyTool implements a langchain tool that executes via the MCP proxy
type ProxyTool struct {
	name        string
	description string
	inputSchema map[string]any
	mcpName     string
	executor    mcpExecutor
}

// NewProxyTool creates a new proxy tool from a tool definition
func NewProxyTool(toolDef mcp.ToolDefinition, proxyClient *mcp.Client) tools.Tool {
	return &ProxyTool{
		name:        toolDef.Name,
		description: toolDef.Description,
		inputSchema: toolDef.InputSchema,
		mcpName:     toolDef.MCPName,
		executor:    mcptools.NewExecutor(proxyClient),
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
	if err := t.validateArguments(ctx, args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	// Execute the tool via proxy
	result, err := t.executor.Execute(ctx, t.mcpName, t.name, args)
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
			return "", fmt.Errorf("failed to marshal tool result: %w", err)
		}
		return string(b), nil
	}
}

// ArgsType returns the input schema to allow the registry to expose parameters
func (t *ProxyTool) ArgsType() any {
	return t.inputSchema
}

// MCPName returns the MCP server ID that provides this tool
func (t *ProxyTool) MCPName() string {
	return t.mcpName
}

// ParameterSchema returns the tool's argument schema for function definitions.
func (t *ProxyTool) ParameterSchema() map[string]any {
	if len(t.inputSchema) == 0 {
		return nil
	}
	copied, err := core.DeepCopy(t.inputSchema)
	if err != nil {
		return core.CloneMap(t.inputSchema)
	}
	return copied
}

// validateArguments validates the provided arguments against the tool's input schema
func (t *ProxyTool) validateArguments(ctx context.Context, args map[string]any) error {
	// Skip validation if no schema is defined
	if len(t.inputSchema) == 0 {
		return nil
	}

	// Convert map[string]any to schema.Schema and use the engine/schema package
	toolSchema := schema.Schema(t.inputSchema)
	validator := schema.NewParamsValidator(args, &toolSchema, fmt.Sprintf("tool:%s", t.name))

	return validator.Validate(ctx)
}
