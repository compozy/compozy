package tools

import (
	"context"
	"errors"
)

// ToolCaller is the minimal contract needed to invoke MCP tools.
type ToolCaller interface {
	CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (any, error)
}

// ErrExecutorNotConfigured is returned when the executor is not properly initialized.
var ErrExecutorNotConfigured = errors.New("mcp executor not configured")

// Executor executes MCP tools through an underlying client.
type Executor struct {
	client ToolCaller
}

// NewExecutor constructs an Executor bound to any ToolCaller.
func NewExecutor(client ToolCaller) *Executor {
	return &Executor{client: client}
}

// Execute invokes the remote tool via the MCP client.
func (e *Executor) Execute(ctx context.Context, serverID, toolName string, args map[string]any) (any, error) {
	if e == nil || e.client == nil {
		return nil, ErrExecutorNotConfigured
	}
	return e.client.CallTool(ctx, serverID, toolName, args)
}
