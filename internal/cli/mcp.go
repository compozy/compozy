package cli

import "github.com/spf13/cobra"

const (
	mcpScopeValue     = "Scope"
	mcpScopeKey       = "scope"
	mcpWorkspaceIDKey = "workspace_id"
)

func newMCPCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   mcpAuthMCPKey,
		Short: "Manage MCP integrations",
	}
	cmd.AddCommand(newMCPInstallCommand(deps))
	cmd.AddCommand(newMCPAuthorizeCommand(deps))
	cmd.AddCommand(newMCPAuthCommand(deps))
	cmd.AddCommand(newMCPServeCommand(deps))
	return cmd
}
