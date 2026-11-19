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
	err := c.doJSON(ctx, http.MethodPut, path, settingsMCPAuthQuery(target), body, &result)
	return result, err
}

func newMCPEnabledCommand(deps commandDeps, enabled bool) *cobra.Command {
	verb, state := "disable", mcpOffState
	if enabled {
		verb, state = "enable", "on"
	}
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser)}
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
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}
