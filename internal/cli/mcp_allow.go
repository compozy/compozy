package cli

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newMCPAllowCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, digest string
	var revoke bool
	cmd := &cobra.Command{Use: "allow <server>", Short: "Allow or revoke an exact project MCP definition", Args: exactOneNonBlankArg(), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		resolved, err := resolveCommandWorkspace(cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef})
		if err != nil {
			return err
		}
		if resolved.ID == "" {
			return fmt.Errorf("cli: --workspace is required")
		}

		approvalClient, ok := client.(MCPApprovalClient)
		if !ok {
			return fmt.Errorf("cli: MCP approvals are unavailable")
		}
		profiles, _, err := profileResolutionClientFromDeps(deps)
		if err != nil {
			return err
		}
		lens, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
		if err != nil {
			return err
		}
		profile := lens.Profile.Name
		label := resolved.Detail.Workspace.Name
		if label == "" {
			label = resolved.ID
		}

		body := contract.SettingsMCPApprovalRequest{WorkspaceID: resolved.ID, Profile: profile, DefinitionDigest: digest}
		if !revoke && body.DefinitionDigest == "" {
			scope := contract.SettingsLayeredScopeProfile
			collection, listErr := client.ListSettingsMCPServers(cmd.Context(), scope, resolved.ID, profile)
			if listErr != nil {
				return listErr
			}
			matches := 0
			for _, server := range collection.MCPServers {
				if server.Name != args[0] || server.Owner != "" && server.Owner != "manual" {
					continue
				}
				matches++
				if matches > 1 {
					return fmt.Errorf("cli: multiple MCP declarations named %s; use --digest to select the observed definition", args[0])
				}
				body.DefinitionDigest = server.Approval.DefinitionDigest
				if _, err = fmt.Fprintf(cmd.ErrOrStderr(), "%s · %s · %s %v\n", server.Name, server.Transport, server.Command, server.Args); err != nil {
					return err
				}
			}
		}
		result, err := approvalClient.SetSettingsMCPApproval(cmd.Context(), args[0], body, revoke)
		if err != nil {
			return err
		}
		if revoke {
			result.Approval = contract.SettingsMCPApprovalPayload{ServerName: args[0], WorkspaceID: body.WorkspaceID, Profile: profile, State: "waiting"}
		}
		return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
			suffix := "Sessions can use it from their next message."
			if revoke {
				suffix = "Its tools stop working on the next call."
			}
			return fmt.Sprintf("%s %s in %s for profile %s. %s\n", map[bool]string{true: "Revoked", false: "Allowed"}[revoke], args[0], label, profile, suffix), nil
		}})
	}}
	cmd.Flags().StringVar(&workspaceRef, "workspace", "", "Project workspace (ID, name, or path)")
	cmd.Flags().StringVar(&digest, "digest", "", "Approve only this definition digest")
	cmd.Flags().BoolVar(&revoke, "revoke", false, "Revoke project MCP consent")
	return cmd
}
