package settings

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/config"
)

// effectiveMCPSourceServers seeds the next config overlay with its actual parent definition.
// Sidecars already replace full definitions; TOML overlays must retain inherited fields.
func effectiveMCPSourceServers(sources map[string][]mcpSourceEntry) []config.MCPServer {
	servers := make([]config.MCPServer, 0, len(sources))
	for _, entries := range sources {
		if len(entries) > 0 {
			servers = append(servers, entries[len(entries)-1].Server)
		}
	}
	return servers
}

func (s *service) RecordWorkspaceMCPWrite(ctx context.Context, w WorkspaceMCPWrite) (string, error) {
	if s.workspaceMCPApprovals == nil {
		if w.Write == nil {
			return "", fmt.Errorf("settings: MCP write is required")
		}
		if err := w.Write(); err != nil {
			return "", err
		}
		return "MCP server saved but approval is unavailable; waiting for approval", nil
	}
	return s.workspaceMCPApprovals.RecordWorkspaceMCPWrite(ctx, w)
}

func (s *service) waitingMCPApprovalError(ctx context.Context, req MCPAuthTargetRequest) error {
	label := req.WorkspaceID
	if s.workspaceResolver != nil && label != "" {
		if workspace, err := s.workspaceResolver.Resolve(ctx, label); err == nil {
			label = workspace.Name
		}
	}
	return WaitingMCPApprovalError(req.Name, label)
}
