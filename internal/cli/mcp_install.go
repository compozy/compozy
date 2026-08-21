package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

type mcpInstallFlags struct {
	name        string
	scope       string
	workspaceID string
	setValues   []string
	secretIDs   []string
	vaultRefs   []string
}

func newMCPInstallCommand(deps commandDeps) *cobra.Command {
	var flags mcpInstallFlags
	cmd := &cobra.Command{
		Use:   installCommandKey + " <entry>",
		Short: "Install a curated MCP server",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request := InstallSettingsMCPServerRequest{
				EntryID:     strings.TrimSpace(args[0]),
				Name:        strings.TrimSpace(flags.name),
				Scope:       contract.SettingsLayeredScopeKind(strings.TrimSpace(flags.scope)),
				WorkspaceID: strings.TrimSpace(flags.workspaceID),
			}
			if request.Name != "" {
				if err := compozyconfig.ValidateMCPServerName(request.Name); err != nil {
					return fmt.Errorf("cli: invalid MCP server name: %w", err)
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if request.Scope == contract.SettingsLayeredScopeWorkspace {
				resolution, err := resolveCommandWorkspace(
					cmd.Context(),
					cmd,
					deps,
					client,
					workspaceResolutionRequest{FlagRef: request.WorkspaceID},
				)
				if err != nil {
					return err
				}
				request.WorkspaceID = resolution.ID
			}
			if request.Scope == contract.SettingsLayeredScopeProfile {
				profiles, _, err := profileResolutionClientFromDeps(deps)
				if err != nil {
					return err
				}
				resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
				if err != nil {
					return err
				}
				request.Profile = strings.TrimSpace(resolution.Profile.Name)
			}
			if err := validateMCPInstallScope(request.Scope, request.WorkspaceID, request.Profile); err != nil {
				return err
			}
			secretReader := newMCPInstallSecretReader(
				cmd.InOrStdin(),
				cmd.ErrOrStderr(),
				deps.inputIsTerminal,
			)
			inputs, err := mcpInstallInputs(flags.setValues, flags.secretIDs, flags.vaultRefs, secretReader.Read)
			if err != nil {
				return err
			}
			request.Values = &contract.SettingsMCPCatalogInstallValuesPayload{
				Inputs: inputs,
			}
			response, err := client.InstallSettingsMCPServer(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, mcpInstallBundle(&response))
		},
	}
	bindMCPInstallFlags(cmd, &flags)
	return cmd
}

func bindMCPInstallFlags(cmd *cobra.Command, flags *mcpInstallFlags) {
	cmd.Flags().StringVar(&flags.name, "name", "", "Override the installed MCP server name")
	cmd.Flags().StringVar(
		&flags.scope,
		mcpScopeKey,
		"",
		"Install scope override: user, profile, or workspace (defaults to the catalog entry)",
	)
	cmd.Flags().
		StringVar(&flags.workspaceID, "workspace", "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringArrayVar(
		&flags.setValues,
		mcpValueInputFlag,
		nil,
		"Set one catalog input as ID=VALUE (repeatable)",
	)
	cmd.Flags().StringArrayVar(
		&flags.secretIDs,
		"secret",
		nil,
		"Read one secret catalog input from stdin or a hidden terminal prompt (repeatable)",
	)
	cmd.Flags().StringArrayVar(&flags.vaultRefs, "vault-ref", nil, "Bind a feed field as KEY=vault:mcp/...")
}
