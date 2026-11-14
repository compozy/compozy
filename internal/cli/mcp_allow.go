package cli

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newMCPAllowCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, digest string
	var revoke bool
	cmd := &cobra.Command{
		Use:   "allow <server>",
		Short: "Allow or revoke an exact project MCP definition",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			resolved, err := resolveCommandWorkspace(
				cmd.Context(),
				cmd,
				deps,
				client,
				workspaceResolutionRequest{FlagRef: workspaceRef},
			)
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

			body := contract.SettingsMCPApprovalRequest{
				WorkspaceID:      resolved.ID,
				Profile:          profile,
				DefinitionDigest: digest,
			}
			if !revoke && body.DefinitionDigest == "" {
				if err := resolveMCPApprovalDigest(cmd, client, args[0], &body); err != nil {
					return err
				}
			}
			result, err := approvalClient.SetSettingsMCPApproval(cmd.Context(), args[0], body, revoke)
			if err != nil {
				return err
			}
			if revoke {
				result.Approval = contract.SettingsMCPApprovalPayload{
					ServerName:  args[0],
					WorkspaceID: body.WorkspaceID,
					Profile:     profile,
					State:       "waiting",
				}
			}
			return writeMCPApprovalOutput(cmd, result, args[0], label, profile, revoke)
		},
	}
	cmd.Flags().StringVar(&workspaceRef, workspaceSkillSource, "", "Project workspace (ID, name, or path)")
	cmd.Flags().StringVar(&digest, "digest", "", "Approve only this definition digest")
	cmd.Flags().BoolVar(&revoke, "revoke", false, "Revoke project MCP consent")
	return cmd
}

func resolveMCPApprovalDigest(
	cmd *cobra.Command, client DaemonClient, name string, body *contract.SettingsMCPApprovalRequest,
) error {
	scope := contract.SettingsLayeredScopeProfile
	collection, listErr := client.ListSettingsMCPServers(cmd.Context(), scope, body.WorkspaceID, body.Profile)
	if listErr != nil {
		return listErr
	}
	matches := 0
	for _, server := range collection.MCPServers {
		if server.Name != name || server.Owner != "" && server.Owner != "manual" {
			continue
		}
		matches++
		if matches > 1 {
			return fmt.Errorf(
				"cli: multiple MCP declarations named %s; use --digest to select the observed definition",
				name,
			)
		}
		body.DefinitionDigest = server.Approval.DefinitionDigest
		if _, err := fmt.Fprintf(
			cmd.ErrOrStderr(),
			"%s · %s · %s %v\n",
			server.Name,
			server.Transport,
			server.Command,
			server.Args,
		); err != nil {
			return err
		}
	}
	return nil
}

func formatMCPApprovalOutput(name, label, profile string, revoke bool) string {
	suffix := "Sessions can use it from their next message."
	if revoke {
		suffix = "Its tools stop working on the next call."
	}
	return fmt.Sprintf(
		"%s %s in %s for profile %s. %s\n",
		map[bool]string{true: "Revoked", false: "Allowed"}[revoke],
		name,
		label,
		profile,
		suffix,
	)
}

func writeMCPApprovalOutput(
	cmd *cobra.Command, result contract.SettingsMCPApprovalResponse, name, label, profile string, revoke bool,
) error {
	return writeCommandOutput(cmd, outputBundle{jsonValue: result,
		jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
		toon: func() (string, error) {
			return renderToonObject(
				"approval",
				[]string{"server_name", automationWorkspaceIDKey, profileFlagName, appTokenStateFlag},
				[]string{
					result.Approval.ServerName,
					result.Approval.WorkspaceID,
					result.Approval.Profile,
					result.Approval.State,
				},
			), nil
		}, human: func() (string, error) {
			return formatMCPApprovalOutput(name, label, profile, revoke), nil
		}})
}
