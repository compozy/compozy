package session

import (
	"context"

	"github.com/compozy/compozy/internal/config"
)

// MCPConsentFilter excludes waiting definitions before a runtime receives launch options.
type MCPConsentFilter func(context.Context, string, string, []config.MCPServer) ([]config.MCPServer, error)

func WithMCPConsentFilter(filter MCPConsentFilter) Option {
	return func(m *Manager) { m.mcpConsentFilter = filter }
}
