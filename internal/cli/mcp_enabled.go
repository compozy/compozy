package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type mcpEnablementClient interface {
	SetSettingsMCPServerEnabled(
		context.Context,
		SettingsMCPAuthTarget,
		bool,
	) (contract.SettingsMCPServerResponse, error)
}

func (c *daemonClient) SetSettingsMCPServerEnabled(
	ctx context.Context, target SettingsMCPAuthTarget, enabled bool,
) (contract.SettingsMCPServerResponse, error) {
	var result contract.SettingsMCPServerResponse
	path := "/api/settings/mcp-servers/" + url.PathEscape(strings.TrimSpace(target.Name)) + "/enabled"
	body := contract.SettingsMCPServerEnabledRequest{Enabled: enabled}
	query := settingsMCPAuthQuery(target)
	if target.DefinitionSource != "" {
		query.Set("definition_source", target.DefinitionSource)
	}
	if err := c.doJSON(ctx, http.MethodPut, path, query, body, &result); err != nil {
		return result, fmt.Errorf("set MCP server %q enabled=%t: %w", target.Name, enabled, err)
	}
	return result, nil
}

func newMCPEnabledCommand(deps commandDeps, enabled bool) *cobra.Command {
	verb, state := "disable", mcpOffState
	if enabled {
		verb, state = "enable", "on"
	}
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser), workspaceDefaultProfile: true}
	var definitionSource string
	cmd := &cobra.Command{
		Use:   verb + " <name>",
		Short: "Turn a configured MCP server " + state,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			writer, ok := client.(mcpEnablementClient)
			if !ok {
				return errors.New("settings MCP enablement client unavailable")
			}
			resolved, err := opts.resolveTarget(cmd, deps, client)
			if err != nil {
				return err
			}
			target, err := resolved.target(args[0])
			if err != nil {
				return err
			}
			target.DefinitionSource = definitionSource
			result, err := writer.SetSettingsMCPServerEnabled(cmd.Context(), target, enabled)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: result,
				human:     func() (string, error) { return fmt.Sprintf("%s: %s\n", result.Server.Name, state), nil },
			})
		},
	}
	cmd.Flags().StringVar(&definitionSource, "definition-source", "", "Declaring file for a same-name MCP server")
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}
