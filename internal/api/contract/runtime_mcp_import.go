package contract

import "github.com/compozy/compozy/internal/runtimemcp"

type RuntimeMCPImportRequest struct {
	Scope       SettingsScopeKind `json:"scope,omitempty"`
	WorkspaceID string            `json:"workspace_id,omitempty"`
	Profile     string            `json:"profile,omitempty"`
	Name        string            `json:"name,omitempty"`
}

type RuntimeMCPImportResponse struct {
	MCPServer SettingsMCPServerItemPayload `json:"mcp_server"`
	Source    runtimemcp.Server            `json:"source"`
	Notes     []string                     `json:"notes"`
	Partial   bool                         `json:"partial"`
	Warnings  []string                     `json:"warnings,omitempty"`
}
