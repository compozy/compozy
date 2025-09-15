package mcpproxy

import "context"

// Storage defines the backend-neutral interface for persisting MCP data.
// Implementations must avoid leaking driver-specific types in method
// signatures and translate backend-specific errors into neutral errors.
//
// Notes:
// - Keep method set stable to preserve public API.
// - Drivers live in dedicated files and MUST NOT be imported here.
type Storage interface {
	SaveMCP(ctx context.Context, def *MCPDefinition) error
	LoadMCP(ctx context.Context, name string) (*MCPDefinition, error)
	DeleteMCP(ctx context.Context, name string) error
	ListMCPs(ctx context.Context) ([]*MCPDefinition, error)
	SaveStatus(ctx context.Context, status *MCPStatus) error
	LoadStatus(ctx context.Context, name string) (*MCPStatus, error)
	Close() error
}
