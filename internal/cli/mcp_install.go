package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/spf13/cobra"
)

type mcpInstallFlags struct {
	name                      string
	scope                     string
	workspaceID               string
	valueKeys                 []string
	vaultRefs                 []string
	oauthClientSecretValue    bool
	oauthClientSecretVaultRef string
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
				Scope:       contract.SettingsWorkspaceScopeKind(strings.TrimSpace(flags.scope)),
				WorkspaceID: strings.TrimSpace(flags.workspaceID),
			}
			if request.Name != "" {
				if err := aghconfig.ValidateMCPServerName(request.Name); err != nil {
					return fmt.Errorf("cli: invalid MCP server name: %w", err)
				}
			}
			if err := validateMCPInstallScope(request.Scope, request.WorkspaceID); err != nil {
				return err
			}
			secretReader := newMCPInstallSecretReader(
				cmd.InOrStdin(),
				cmd.ErrOrStderr(),
				deps.inputIsTerminal,
			)
			env, err := mcpInstallEnvInputs(flags.valueKeys, flags.vaultRefs, secretReader.Read)
			if err != nil {
				return err
			}
			oauthClientSecret, err := mcpInstallOAuthClientSecretInput(
				flags.oauthClientSecretValue,
				cmd.Flags().Changed(mcpOAuthVaultRefInputFlag),
				flags.oauthClientSecretVaultRef,
				secretReader.Read,
			)
			if err != nil {
				return err
			}
			request.Values = &contract.SettingsMCPCatalogInstallValuesPayload{
				Env:               env,
				OAuthClientSecret: oauthClientSecret,
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
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
		string(contract.SettingsWorkspaceScopeGlobal),
		"Install scope: global or workspace",
	)
	cmd.Flags().StringVar(&flags.workspaceID, "workspace", "", "Workspace ID for workspace scope")
	cmd.Flags().StringArrayVar(
		&flags.valueKeys,
		mcpValueInputFlag,
		nil,
		"Read one feed field value from stdin or a hidden terminal prompt (repeatable)",
	)
	cmd.Flags().StringArrayVar(&flags.vaultRefs, "vault-ref", nil, "Bind a feed field as KEY=vault:mcp/...")
	cmd.Flags().BoolVar(
		&flags.oauthClientSecretValue,
		mcpOAuthValueInputFlag,
		false,
		"Read the write-only OAuth client secret from stdin or a hidden terminal prompt",
	)
	cmd.Flags().StringVar(
		&flags.oauthClientSecretVaultRef,
		mcpOAuthVaultRefInputFlag,
		"",
		"Bind the OAuth client secret to an existing vault:mcp/... ref",
	)
}
