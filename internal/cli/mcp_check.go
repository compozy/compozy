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

func (c *daemonClient) CheckSettingsMCPServer(
	ctx context.Context, target SettingsMCPAuthTarget,
) (contract.SettingsMCPServerItemPayload, error) {
	var result contract.SettingsMCPServerItemPayload
	path := "/api/settings/mcp-servers/" + url.PathEscape(strings.TrimSpace(target.Name)) + "/check"
	err := c.doJSON(ctx, http.MethodPost, path, settingsMCPAuthQuery(target), nil, &result)
	return result, err
}

func newMCPCheckCommand(deps commandDeps) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser)}
	cmd := &cobra.Command{
		Use:     "check <name>",
		Short:   "Check one configured MCP server again",
		Example: "  compozy mcp check linear\n  compozy mcp check docs --scope workspace --workspace my-project -o json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			resolved, err := opts.resolveTarget(cmd, deps, client)
			if err != nil {
				return err
			}
			target, err := resolved.target(args[0])
			if err != nil {
				return err
			}
			checker, ok := client.(interface {
				CheckSettingsMCPServer(context.Context, SettingsMCPAuthTarget) (contract.SettingsMCPServerItemPayload, error)
			})
			if !ok {
				return errors.New("settings MCP check client unavailable")
			}
			result, err := checker.CheckSettingsMCPServer(cmd.Context(), target)
			if err != nil {
				return err
			}
			state := "unknown"
			if result.RuntimeStatus != nil {
				state = result.RuntimeStatus.State
			}
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: result,
				human:     func() (string, error) { return fmt.Sprintf("%s: %s\n", result.Name, state), nil },
			})
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}
