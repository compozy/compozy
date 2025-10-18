package tools

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/mcp"
)

// Executor executes MCP tools through an underlying client.
type Executor struct {
	client *mcp.Client
}

// NewExecutor constructs an Executor bound to the provided MCP client.
func NewExecutor(client *mcp.Client) *Executor {
	return &Executor{client: client}
}

// Execute invokes the remote tool via the MCP client.
func (e *Executor) Execute(ctx context.Context, serverID, toolName string, args map[string]any) (any, error) {
	if e == nil || e.client == nil {
		return nil, fmt.Errorf("mcp executor not configured")
	}
	return e.client.CallTool(ctx, serverID, toolName, args)
}
