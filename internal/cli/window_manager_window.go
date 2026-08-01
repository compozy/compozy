package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

const (
	windowCommandKey = "window"
	windowCloseKey   = "close"
)

type windowOpenFlagValues struct {
	windowID    string
	app         string
	instanceKey string
	desktopID   string
	restoreID   string
	stackTarget string
	rectRaw     string
	pathname    string
	searchJSON  string
	insertTiled bool
}

func newWindowCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: windowCommandKey, Short: "Manage windows in virtual desktops", Args: cobra.NoArgs}
	cmd.AddCommand(newWindowListCommand(deps))
	cmd.AddCommand(newWindowOpenCommand(deps))
	cmd.AddCommand(newWindowNavigateCommand(deps))
	cmd.AddCommand(newWindowCloseCommand(deps))
	cmd.AddCommand(newWindowGroupCommand(deps))
	cmd.AddCommand(newWindowActivateCommand(deps))
	cmd.AddCommand(newWindowPinCommand(deps, true))
	cmd.AddCommand(newWindowPinCommand(deps, false))
	cmd.AddCommand(newWindowReopenCommand(deps))
	cmd.AddCommand(newWindowFocusCommand(deps))
	cmd.AddCommand(newWindowMoveCommand(deps))
	cmd.AddCommand(newWindowResizeCommand(deps))
	cmd.AddCommand(newWindowSwapCommand(deps))
	cmd.AddCommand(newWindowFloatCommand(deps))
	cmd.AddCommand(newWindowZoomCommand(deps))
	return cmd
}

func newWindowListCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use: agentKernelListKey, Short: "List windows across workspace desktops", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := windowManagerClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			snapshot, err := client.GetWindowManagerSnapshot(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerWindowListBundle(snapshot))
		},
	}
	cmd.Flags().
		StringVar(&workspace, windowManagerWorkspaceFlag, "", "Override workspace (ID, name, or path)")
	return cmd
}

func newWindowOpenCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var values windowOpenFlagValues
	cmd := &cobra.Command{
		Use:   windowManagerOpenKey,
		Short: "Open a window or restore a minimized window",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowOpenCommand(cmd, deps, flags, values)
		},
	}
	flags.add(cmd)
	values.addFlags(cmd)
	return cmd
}

func runWindowOpenCommand(
	cmd *cobra.Command,
	deps commandDeps,
	flags windowManagerMutationFlags,
	values windowOpenFlagValues,
) error {
	restoreWindowID, err := optionalWindowManagerID[windowmanager.WindowID](cmd, "restore", values.restoreID)
	if err != nil {
		return err
	}
	if restoreWindowID != nil {
		if err := validateWindowRestoreFlags(cmd); err != nil {
			return err
		}
	}
	spec, err := windowOpenSpec(cmd, values, restoreWindowID)
	if err != nil {
		return err
	}
	request, err := flags.request(
		cmd,
		deps,
		contract.WindowManagerCommandWindowOpen,
		contract.WindowManagerOpenWindowPayload{Window: spec, RestoreWindowID: restoreWindowID},
	)
	if err != nil {
		return err
	}
	result, err := executeWindowManagerCommand(cmd, deps, request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, windowManagerResultBundle(result))
}

func validateWindowRestoreFlags(cmd *cobra.Command) error {
	newWindowFlagsChanged := cmd.Flags().Changed(windowManagerAppFlag) ||
		cmd.Flags().Changed("id") ||
		cmd.Flags().Changed("instance-key") ||
		cmd.Flags().Changed("desktop") ||
		cmd.Flags().Changed(windowManagerRectFlag) ||
		cmd.Flags().Changed(windowManagerPathnameFlag) ||
		cmd.Flags().Changed(windowManagerSearchJSONFlag) ||
		cmd.Flags().Changed("tiled") ||
		cmd.Flags().Changed("stack-target")
	if newWindowFlagsChanged {
		return newWindowManagerCLIValidationError(
			windowManagerCLIValidationConflicting,
			"restore",
			errors.New("cli: --restore cannot be combined with new-window flags"),
		)
	}
	return nil
}

func windowOpenSpec(
	cmd *cobra.Command,
	values windowOpenFlagValues,
	restoreWindowID *windowmanager.WindowID,
) (contract.WindowManagerWindowSpecPayload, error) {
	if restoreWindowID != nil {
		return contract.WindowManagerWindowSpecPayload{}, nil
	}
	app, err := requiredWindowManagerFlag(values.app, windowManagerAppFlag)
	if err != nil {
		return contract.WindowManagerWindowSpecPayload{}, err
	}
	instance, err := optionalWindowManagerID[string](cmd, "instance-key", values.instanceKey)
	if err != nil {
		return contract.WindowManagerWindowSpecPayload{}, err
	}
	stackTarget, err := optionalWindowManagerID[windowmanager.WindowID](cmd, "stack-target", values.stackTarget)
	if err != nil {
		return contract.WindowManagerWindowSpecPayload{}, err
	}
	rect, err := optionalWindowManagerRect(cmd, values.rectRaw)
	if err != nil {
		return contract.WindowManagerWindowSpecPayload{}, err
	}
	var floatingRect windowmanager.NormalizedRect
	if rect != nil {
		floatingRect = *rect
	}
	route, err := requiredWindowManagerRoute(cmd, values.pathname, values.searchJSON)
	if err != nil {
		return contract.WindowManagerWindowSpecPayload{}, err
	}
	return contract.WindowManagerWindowSpecPayload{
		ID:                  windowmanager.WindowID(strings.TrimSpace(values.windowID)),
		App:                 app,
		InstanceKey:         instance,
		Route:               route,
		DesktopID:           windowmanager.DesktopID(strings.TrimSpace(values.desktopID)),
		FloatingRect:        floatingRect,
		InsertTiled:         values.insertTiled,
		StackTargetWindowID: stackTarget,
	}, nil
}

func (values *windowOpenFlagValues) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&values.windowID, "id", "", "Stable window ID; generated when omitted")
	cmd.Flags().StringVar(&values.app, windowManagerAppFlag, "", "Application route identifier")
	cmd.Flags().StringVar(&values.instanceKey, "instance-key", "", "Optional application instance key")
	cmd.Flags().StringVar(
		&values.desktopID,
		"desktop",
		"",
		"Destination desktop; resolved from client focus when omitted",
	)
	cmd.Flags().StringVar(
		&values.restoreID,
		"restore",
		"",
		"Restore this minimized window instead of opening a new one",
	)
	cmd.Flags().StringVar(
		&values.rectRaw,
		windowManagerRectFlag,
		"",
		"Floating rect as x,y,width,height in normalized coordinates",
	)
	cmd.Flags().BoolVar(&values.insertTiled, "tiled", false, "Insert beside client focus when possible")
	cmd.Flags().StringVar(&values.stackTarget, "stack-target", "", "Add the new window to this window's stack")
	addWindowManagerRouteFlags(cmd, &values.pathname, &values.searchJSON)
}

func newWindowCloseCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID string
	var minimize bool
	var scope string
	cmd := &cobra.Command{
		Use: windowCloseKey, Short: "Close or minimize a window and reflow its group", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			windowID, err := requiredWindowManagerFlag(windowID, "id")
			if err != nil {
				return err
			}
			closeScope, err := parseWindowManagerCloseScope(scope)
			if err != nil {
				return err
			}
			if minimize && closeScope != windowmanager.CloseScopeTab {
				return newWindowManagerCLIValidationError(
					windowManagerCLIValidationConflicting,
					"scope",
					errors.New("cli: --minimize cannot be combined with a non-tab --scope"),
				)
			}
			request, err := flags.request(
				cmd,
				deps,
				contract.WindowManagerCommandWindowClose,
				contract.WindowManagerCloseWindowPayload{
					WindowID: windowmanager.WindowID(windowID), Minimize: minimize, Scope: closeScope,
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
	cmd.Flags().BoolVar(&minimize, "minimize", false, "Keep the window available for restore")
	cmd.Flags().StringVar(&scope, "scope", "tab", "Close scope: tab, group, others, or right")
	return cmd
}

func parseWindowManagerCloseScope(raw string) (windowmanager.CloseScope, error) {
	switch normalized := strings.TrimSpace(raw); normalized {
	case "", "tab":
		return windowmanager.CloseScopeTab, nil
	case string(windowmanager.CloseScopeGroup):
		return windowmanager.CloseScopeGroup, nil
	case string(windowmanager.CloseScopeOthers):
		return windowmanager.CloseScopeOthers, nil
	case string(windowmanager.CloseScopeRight):
		return windowmanager.CloseScopeRight, nil
	default:
		return "", newWindowManagerCLIValidationError(
			windowManagerCLIValidationInvalidValue,
			"scope",
			fmt.Errorf("cli: unsupported --scope %q", raw),
		)
	}
}

func newWindowFocusCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var windowID, direction string
	cmd := &cobra.Command{
		Use:   windowManagerFocusKey,
		Short: "Focus a window by ID or direction for one connected client",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requiredWindowManagerClient(flags); err != nil {
				return err
			}
			byID := strings.TrimSpace(windowID) != ""
			byDirection := strings.TrimSpace(direction) != ""
			if byID == byDirection {
				return newWindowManagerCLIValidationError(
					windowManagerCLIValidationConflicting,
					"focus_target",
					errors.New("cli: exactly one of --id or --direction is required"),
				)
			}
			var windowIDPtr *windowmanager.WindowID
			if byID {
				value := windowmanager.WindowID(strings.TrimSpace(windowID))
				windowIDPtr = &value
			}
			focusDirection := windowmanager.FocusDirection(strings.TrimSpace(direction))
			if byDirection && !isWindowManagerFocusDirection(focusDirection) {
				return fmt.Errorf("cli: unsupported --direction %q", direction)
			}
			request, err := flags.request(
				cmd,
				deps,
				contract.WindowManagerCommandWindowFocus,
				contract.WindowManagerFocusWindowPayload{
					WindowID: windowIDPtr, Direction: focusDirection,
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
	cmd.Flags().StringVar(&direction, "direction", "", "Direction: left, right, up, or down")
	return cmd
}

func isWindowManagerFocusDirection(direction windowmanager.FocusDirection) bool {
	switch direction {
	case windowmanager.FocusLeft, windowmanager.FocusRight, windowmanager.FocusUp, windowmanager.FocusDown:
		return true
	default:
		return false
	}
}
