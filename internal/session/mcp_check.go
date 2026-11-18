package session

import (
	"context"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type MCPChecker interface {
	CheckSessionMCP(context.Context, string, string, string) (MCP, error)
}

var _ MCPChecker = (*Manager)(nil)

func (m *Manager) CheckSessionMCP(ctx context.Context, id, name, owner string) (MCP, error) {
	checker, ok := m.mcpReader.(MCPChecker)
	if !ok {
		return MCP{}, fmt.Errorf("%w: session MCP check unavailable", toolspkg.ErrToolUnavailable)
	}
	return checker.CheckSessionMCP(ctx, id, name, owner)
}
