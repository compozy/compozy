package cli

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

func newWindowResizeCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID, rectRaw string
	cmd := &cobra.Command{
		Use:   windowManagerResizeKey,
		Short: "Resize a window frame; a tiled split member detaches into its own island",
		Example: "  # Output example\n" +
			"  # Set WINDOW_ID to an existing window ID.\n" +
			"  compozy window resize --id \"$WINDOW_ID\" --rect 0.10,0.10,0.80,0.80",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			rect, err := optionalWindowManagerRect(cmd, rectRaw)
			if err != nil {
				return err
			}
			if rect == nil {
				return errors.New("cli: --rect is required")
			}
			request, err := flags.request(
				cmd,
				deps,
				contract.WindowManagerCommandWindowResize,
				contract.WindowManagerResizeWindowPayload{
					WindowID: windowmanager.WindowID(windowID),
					Frame:    *rect,
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
	cmd.Flags().StringVar(&rectRaw, windowManagerRectFlag, "", "Frame as x,y,width,height in normalized coordinates")
	return cmd
}
