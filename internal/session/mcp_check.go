package session

import (
	"context"
	"errors"
)

type MCPChecker interface {
	CheckSessionMCP(context.Context, string, string, string) (MCP, error)
}

var _ MCPChecker = (*Manager)(nil)

func (m *Manager) CheckSessionMCP(ctx context.Context, id, name, owner string) (MCP, error) {
	checker, ok := m.mcpReader.(MCPChecker)
	if !ok {
		return MCP{}, errors.New("session: MCP check unavailable")
	}
	return checker.CheckSessionMCP(ctx, id, name, owner)
}
