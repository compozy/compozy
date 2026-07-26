package cli

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

func newWindowNavigateCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID, pathname, searchJSON string
	cmd := &cobra.Command{
		Use: windowManagerNavigateKey, Short: "Persist a window's internal application route", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			route, err := requiredWindowManagerRoute(cmd, pathname, searchJSON)
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandWindowNavigate,
				contract.WindowManagerNavigateWindowPayload{
					WindowID: windowmanager.WindowID(windowID),
					Route:    route,
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
	addWindowManagerRouteFlags(cmd, &pathname, &searchJSON)
	return cmd
}
