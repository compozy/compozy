package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

const windowGroupKey = "group"

func newWindowGroupCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var target, windowsRaw string
	var insertIndex int
	cmd := &cobra.Command{
		Use: windowGroupKey, Short: "Deprecated: place windows in adjacent tabs; removed in v0.8.0", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := requiredWindowManagerFlag(target, "target")
			if err != nil {
				return err
			}
			windowIDs, err := parseWindowManagerWindowIDs(windowsRaw)
			if err != nil {
				return err
			}
			var insertIndexPtr *int
			if cmd.Flags().Changed("insert-index") {
				insertIndexPtr = &insertIndex
			}
			request, err := flags.request(
				cmd,
				deps,
				contract.WindowManagerCommandWindowStackGroup,
				contract.WindowManagerGroupWindowsPayload{
					TargetWindowID: windowmanager.WindowID(target),
					WindowIDs:      windowIDs,
					InsertIndex:    insertIndexPtr,
				},
			)
			if err != nil {
				return err
			}
			result, err := executeWindowManagerCommand(cmd, deps, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(&result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&target, "target", "", "Target window ID")
	cmd.Flags().
		StringVar(&windowsRaw, "windows", "", "Deprecated: comma-separated IDs to place in adjacent tabs; "+
			"use window move; removed in v0.8.0")
	cmd.Flags().IntVar(&insertIndex, "insert-index", 0, "Deprecated: ignored insertion index; removed in v0.8.0")
	return cmd
}

func parseWindowManagerWindowIDs(raw string) ([]windowmanager.WindowID, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, newWindowManagerCLIValidationError(
			windowManagerCLIValidationRequired,
			"windows",
			errors.New("cli: --windows is required"),
		)
	}
	parts := strings.Split(raw, ",")
	result := make([]windowmanager.WindowID, 0, len(parts))
	seen := make(map[windowmanager.WindowID]struct{}, len(parts))
	for _, part := range parts {
		windowID := windowmanager.WindowID(strings.TrimSpace(part))
		if windowID == "" {
			return nil, newWindowManagerCLIValidationError(
				windowManagerCLIValidationInvalidValue,
				"windows",
				errors.New("cli: --windows must contain non-empty comma-separated IDs"),
			)
		}
		if _, duplicate := seen[windowID]; duplicate {
			return nil, newWindowManagerCLIValidationError(
				windowManagerCLIValidationInvalidValue,
				"windows",
				errors.New("cli: --windows must not contain duplicate IDs"),
			)
		}
		seen[windowID] = struct{}{}
		result = append(result, windowID)
	}
	return result, nil
}

func newWindowActivateCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	cmd := &cobra.Command{
		Use: "activate <window-id>", Short: "Activate the window's tab", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeWindowTabCommand(
				cmd,
				deps,
				flags,
				contract.WindowManagerCommandWindowTabActivate,
				contract.WindowManagerSetStackActivePayload{WindowID: windowmanager.WindowID(args[0])},
			)
		},
	}
	flags.add(cmd)
	return cmd
}

func newWindowPinCommand(deps commandDeps, pinned bool) *cobra.Command {
	var flags windowManagerMutationFlags
	verb := cliUnpinVerb
	short := "Unpin the window's tab"
	if pinned {
		verb = cliPinVerb
		short = "Pin the window's tab"
	}
	cmd := &cobra.Command{
		Use: verb + " <window-id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeWindowTabCommand(
				cmd,
				deps,
				flags,
				contract.WindowManagerCommandWindowPin,
				contract.WindowManagerPinWindowPayload{
					WindowID: windowmanager.WindowID(args[0]),
					Pinned:   pinned,
				},
			)
		},
	}
	flags.add(cmd)
	return cmd
}

func newWindowReopenCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	cmd := &cobra.Command{
		Use: "reopen", Short: "Restore the newest closed window entry", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeWindowTabCommand(
				cmd,
				deps,
				flags,
				contract.WindowManagerCommandWindowReopen,
				contract.WindowManagerReopenWindowPayload{},
			)
		},
	}
	flags.add(cmd)
	return cmd
}

func executeWindowTabCommand(
	cmd *cobra.Command,
	deps commandDeps,
	flags windowManagerMutationFlags,
	commandID contract.WindowManagerCommandID,
	payload any,
) error {
	request, err := flags.request(cmd, deps, commandID, payload)
	if err != nil {
		return err
	}
	result, err := executeWindowManagerCommand(cmd, deps, request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, windowManagerResultBundle(&result))
}

func newWindowReorderCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var index int
	cmd := &cobra.Command{
		Use: "reorder <window-id>", Short: "Reorder the window's tab", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("index") {
				return errors.New("cli: --index is required")
			}
			return executeWindowTabCommand(
				cmd,
				deps,
				flags,
				contract.WindowManagerCommandWindowTabReorder,
				contract.WindowManagerReorderStackPayload{
					WindowID: windowmanager.WindowID(strings.TrimSpace(args[0])),
					Index:    index,
				},
			)
		},
	}
	flags.add(cmd)
	cmd.Flags().IntVar(&index, "index", 0, "Zero-based tab index, clamped to the tab's pinned or unpinned range")
	return cmd
}
