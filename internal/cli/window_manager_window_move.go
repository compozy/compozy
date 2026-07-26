package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

type windowMoveFlagValues struct {
	windowID    string
	destination string
	targetID    string
	placement   string
	rectRaw     string
	moveGroup   bool
}

func newWindowMoveCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var values windowMoveFlagValues
	cmd := &cobra.Command{
		Use:   windowManagerMoveKey,
		Short: "Move a window or tiled group to an explicit destination",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowMoveCommand(cmd, deps, flags, values)
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&values.windowID, "id", "", "Window ID")
	cmd.Flags().StringVar(&values.destination, "desktop", "", "Destination desktop ID")
	cmd.Flags().StringVar(&values.targetID, automationTargetKey, "", "Target window for structural placement")
	cmd.Flags().StringVar(
		&values.placement,
		"placement",
		string(windowmanager.DropFloating),
		"Placement: floating, before, after, left, right, top, bottom, or center",
	)
	cmd.Flags().StringVar(&values.rectRaw, windowManagerRectFlag, "", "Floating rect as x,y,width,height")
	cmd.Flags().BoolVar(&values.moveGroup, "group", false, "Move the complete tiled group")
	return cmd
}

func runWindowMoveCommand(
	cmd *cobra.Command,
	deps commandDeps,
	flags windowManagerMutationFlags,
	values windowMoveFlagValues,
) error {
	payload, err := windowMovePayload(cmd, values)
	if err != nil {
		return err
	}
	request, err := flags.request(cmd, contract.WindowManagerCommandWindowMove, payload)
	if err != nil {
		return err
	}
	result, err := executeWindowManagerCommand(cmd, deps, request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, windowManagerResultBundle(result))
}

func windowMovePayload(
	cmd *cobra.Command,
	values windowMoveFlagValues,
) (contract.WindowManagerMoveWindowPayload, error) {
	windowID, err := requiredWindowManagerFlag(values.windowID, "id")
	if err != nil {
		return contract.WindowManagerMoveWindowPayload{}, err
	}
	destination, err := requiredWindowManagerFlag(values.destination, "desktop")
	if err != nil {
		return contract.WindowManagerMoveWindowPayload{}, err
	}
	target, placement, rect, err := windowMovePlacement(cmd, values)
	if err != nil {
		return contract.WindowManagerMoveWindowPayload{}, err
	}
	return contract.WindowManagerMoveWindowPayload{
		WindowID:             windowmanager.WindowID(windowID),
		DestinationDesktopID: windowmanager.DesktopID(destination),
		TargetWindowID:       target,
		Placement:            placement,
		FloatingRect:         rect,
		MoveGroup:            values.moveGroup,
	}, nil
}

func windowMovePlacement(
	cmd *cobra.Command,
	values windowMoveFlagValues,
) (*windowmanager.WindowID, windowmanager.DropPlacement, *windowmanager.NormalizedRect, error) {
	if values.moveGroup {
		for _, flagName := range []string{automationTargetKey, "placement", windowManagerRectFlag} {
			if cmd.Flags().Changed(flagName) {
				return nil, "", nil, newWindowManagerCLIValidationError(
					windowManagerCLIValidationConflicting,
					windowManagerGroupFlag,
					fmt.Errorf("cli: --group cannot be combined with --%s", flagName),
				)
			}
		}
		return nil, "", nil, nil
	}
	placement := windowmanager.DropPlacement(strings.TrimSpace(values.placement))
	if !isWindowManagerDropPlacement(placement) {
		return nil, "", nil, fmt.Errorf("cli: unsupported --placement %q", values.placement)
	}
	target, err := optionalWindowManagerID[windowmanager.WindowID](cmd, automationTargetKey, values.targetID)
	if err != nil {
		return nil, "", nil, err
	}
	if placement != windowmanager.DropFloating && target == nil {
		return nil, "", nil, newWindowManagerCLIValidationError(
			windowManagerCLIValidationRequired,
			automationTargetKey,
			errors.New("cli: --target is required for structural placement"),
		)
	}
	rect, err := optionalWindowManagerRect(cmd, values.rectRaw)
	if err != nil {
		return nil, "", nil, err
	}
	if rect != nil && placement != windowmanager.DropFloating {
		return nil, "", nil, errors.New("cli: --rect is only valid with --placement=floating")
	}
	return target, placement, rect, nil
}

func newWindowSwapCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var firstID, secondID string
	cmd := &cobra.Command{
		Use: "swap", Short: "Swap two window placements", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			firstID, err := requiredWindowManagerFlag(firstID, "first")
			if err != nil {
				return err
			}
			secondID, err := requiredWindowManagerFlag(secondID, "second")
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandWindowSwap,
				contract.WindowManagerSwapWindowsPayload{
					FirstWindowID: windowmanager.WindowID(firstID), SecondWindowID: windowmanager.WindowID(secondID),
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
	cmd.Flags().StringVar(&firstID, "first", "", "First window ID")
	cmd.Flags().StringVar(&secondID, "second", "", "Second window ID")
	return cmd
}

func newWindowFloatCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID, rectRaw string
	cmd := &cobra.Command{
		Use: "float", Short: "Toggle a window between floating and tiled placement", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			rect, err := optionalWindowManagerRect(cmd, rectRaw)
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandWindowToggleFloating,
				contract.WindowManagerToggleFloatingPayload{
					WindowID: windowmanager.WindowID(windowID), FloatingRect: rect,
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
	cmd.Flags().StringVar(&rectRaw, windowManagerRectFlag, "", "Floating rect as x,y,width,height")
	return cmd
}

func newWindowZoomCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID string
	cmd := &cobra.Command{
		Use: "zoom", Short: "Toggle a window's client-specific focus desktop", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requiredWindowManagerClient(flags); err != nil {
				return err
			}
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandWindowZoom,
				contract.WindowManagerZoomWindowPayload{
					WindowID: windowmanager.WindowID(windowID),
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
	return cmd
}

func isWindowManagerDropPlacement(placement windowmanager.DropPlacement) bool {
	switch placement {
	case windowmanager.DropFloating, windowmanager.DropBefore, windowmanager.DropAfter,
		windowmanager.DropLeft, windowmanager.DropRight, windowmanager.DropTop,
		windowmanager.DropBottom, windowmanager.DropCenter:
		return true
	default:
		return false
	}
}
