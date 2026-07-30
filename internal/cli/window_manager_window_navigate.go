package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

func newWindowNavigateCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID, pathname, searchJSON, mode string
	cmd := &cobra.Command{
		Use: windowManagerNavigateKey, Short: "Persist a window's internal application route", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			navigateMode, err := parseWindowManagerNavigateMode(mode)
			if err != nil {
				return err
			}
			var route windowmanager.RouteIntent
			if navigateMode == windowmanager.NavigatePop {
				if cmd.Flags().Changed(windowManagerPathnameFlag) || cmd.Flags().Changed(windowManagerSearchJSONFlag) {
					return newWindowManagerCLIValidationError(
						windowManagerCLIValidationConflicting,
						"mode",
						errors.New("cli: --mode pop cannot be combined with route flags"),
					)
				}
			} else {
				route, err = requiredWindowManagerRoute(cmd, pathname, searchJSON)
				if err != nil {
					return err
				}
			}
			request, err := flags.request(
				cmd,
				deps,
				contract.WindowManagerCommandWindowNavigate,
				contract.WindowManagerNavigateWindowPayload{
					WindowID: windowmanager.WindowID(windowID),
					Route:    route,
					Mode:     navigateMode,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&windowID, "id", "", "Window ID")
	cmd.Flags().StringVar(&mode, "mode", "replace", "Navigation mode: replace, push, or pop")
	addWindowManagerRouteFlags(cmd, &pathname, &searchJSON)
	return cmd
}

func parseWindowManagerNavigateMode(raw string) (windowmanager.NavigateMode, error) {
	switch normalized := strings.TrimSpace(raw); normalized {
	case "", "replace":
		return windowmanager.NavigateReplace, nil
	case string(windowmanager.NavigatePush):
		return windowmanager.NavigatePush, nil
	case string(windowmanager.NavigatePop):
		return windowmanager.NavigatePop, nil
	default:
		return "", newWindowManagerCLIValidationError(
			windowManagerCLIValidationInvalidValue,
			"mode",
			fmt.Errorf("cli: unsupported --mode %q", raw),
		)
	}
}
