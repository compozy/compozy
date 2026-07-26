package cli

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

type windowManagerArrangeResourcePayload struct {
	ResourceID string `json:"resource_id"`
}

func newLayoutArrangeCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var desktopID, arrangement, frameRaw, groupID, resourceID string
	var windowIDs []string
	cmd := &cobra.Command{
		Use:   windowManagerArrangeKey,
		Short: "Arrange explicit windows or apply a declarative layout resource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := resolveWindowManagerArrangePayload(
				cmd,
				desktopID,
				windowIDs,
				arrangement,
				frameRaw,
				groupID,
				resourceID,
			)
			if err != nil {
				return err
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandLayoutArrange,
				payload,
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
	cmd.Flags().StringVar(&desktopID, desktopCommandKey, "", "Desktop ID")
	cmd.Flags().StringArrayVar(&windowIDs, windowCommandKey, nil, "Participant window ID; repeat for each window")
	cmd.Flags().StringVar(
		&arrangement,
		windowManagerArrangementFlag,
		"",
		"Arrangement: horizontal, vertical, grid, or stack",
	)
	cmd.Flags().StringVar(
		&frameRaw,
		windowManagerFrameFlag,
		"",
		"Group frame as x,y,width,height; defaults to the full desktop",
	)
	cmd.Flags().StringVar(&groupID, windowManagerGroupFlag, "", "Stable group ID; generated when omitted")
	cmd.Flags().StringVar(&resourceID, windowManagerResourceFlag, "", "Declarative layout resource ID")
	return cmd
}

func resolveWindowManagerArrangePayload(
	cmd *cobra.Command,
	desktopID string,
	windowIDs []string,
	arrangement string,
	frameRaw string,
	groupID string,
	resourceID string,
) (any, error) {
	if cmd.Flags().Changed(windowManagerResourceFlag) {
		resourceID, err := requiredWindowManagerFlag(resourceID, windowManagerResourceFlag)
		if err != nil {
			return nil, err
		}
		inlineFlags := []string{
			desktopCommandKey,
			windowCommandKey,
			windowManagerArrangementFlag,
			windowManagerFrameFlag,
			windowManagerGroupFlag,
		}
		for _, flagName := range inlineFlags {
			if cmd.Flags().Changed(flagName) {
				return nil, newWindowManagerCLIValidationError(
					windowManagerCLIValidationConflicting,
					windowManagerResourceFlag,
					fmt.Errorf("cli: --resource cannot be combined with --%s", flagName),
				)
			}
		}
		return windowManagerArrangeResourcePayload{ResourceID: resourceID}, nil
	}

	desktopID, err := requiredWindowManagerFlag(desktopID, desktopCommandKey)
	if err != nil {
		return nil, err
	}
	participants, err := requiredWindowManagerIDs(windowIDs, windowCommandKey)
	if err != nil {
		return nil, err
	}
	arrangementValue, err := requiredWindowManagerFlag(arrangement, windowManagerArrangementFlag)
	if err != nil {
		return nil, err
	}
	arrangementKind := windowmanager.Arrangement(arrangementValue)
	if !isWindowManagerArrangement(arrangementKind) {
		return nil, fmt.Errorf("cli: unsupported --arrangement %q", arrangement)
	}
	frame, err := optionalWindowManagerNormalizedRect(cmd, windowManagerFrameFlag, frameRaw)
	if err != nil {
		return nil, err
	}
	var normalizedFrame windowmanager.NormalizedRect
	if frame != nil {
		normalizedFrame = *frame
	}
	return contract.WindowManagerArrangeLayoutPayload{
		DesktopID: windowmanager.DesktopID(desktopID), WindowIDs: participants,
		Arrangement: arrangementKind, Frame: normalizedFrame,
		GroupID: windowmanager.GroupID(strings.TrimSpace(groupID)),
	}, nil
}

func newLayoutResizeCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var splitID string
	var boundary int
	var delta float64
	cmd := &cobra.Command{
		Use: "resize", Short: "Resize one shared split boundary", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			splitID, err := requiredWindowManagerFlag(splitID, "split")
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("boundary") {
				return errors.New("cli: --boundary is required")
			}
			if boundary < 0 {
				return errors.New("cli: --boundary must be nonnegative")
			}
			if !cmd.Flags().Changed("delta") {
				return errors.New("cli: --delta is required")
			}
			if math.IsNaN(delta) || math.IsInf(delta, 0) {
				return errors.New("cli: --delta must be finite")
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandLayoutResize,
				contract.WindowManagerResizeLayoutPayload{
					SplitID: windowmanager.NodeID(splitID), BoundaryIndex: boundary, Delta: delta,
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
	cmd.Flags().StringVar(&splitID, "split", "", "Split node ID")
	cmd.Flags().IntVar(&boundary, "boundary", 0, "Zero-based boundary between adjacent children")
	cmd.Flags().Float64Var(&delta, "delta", 0, "Normalized weight transferred from right to left")
	return cmd
}

func newLayoutBalanceCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var groupID, splitID string
	cmd := &cobra.Command{
		Use: "balance", Short: "Equalize one split or every split in a group", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			groupSet := strings.TrimSpace(groupID) != ""
			splitSet := strings.TrimSpace(splitID) != ""
			if groupSet == splitSet {
				return errors.New("cli: exactly one of --group or --split is required")
			}
			var group *windowmanager.GroupID
			if groupSet {
				value := windowmanager.GroupID(strings.TrimSpace(groupID))
				group = &value
			}
			var split *windowmanager.NodeID
			if splitSet {
				value := windowmanager.NodeID(strings.TrimSpace(splitID))
				split = &value
			}
			request, err := flags.request(
				cmd,
				contract.WindowManagerCommandLayoutBalance,
				contract.WindowManagerBalanceLayoutPayload{
					GroupID: group, SplitID: split,
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
	cmd.Flags().StringVar(&groupID, windowManagerGroupFlag, "", "Layout group ID")
	cmd.Flags().StringVar(&splitID, "split", "", "Split node ID")
	return cmd
}

func newLayoutUndoCommand(deps commandDeps) *cobra.Command {
	return newLayoutHistoryCommand(
		deps,
		"undo",
		"Undo the latest workspace layout operation",
		contract.WindowManagerCommandLayoutUndo,
		contract.WindowManagerUndoLayoutPayload{},
	)
}

func newLayoutRedoCommand(deps commandDeps) *cobra.Command {
	return newLayoutHistoryCommand(
		deps,
		"redo",
		"Redo the latest undone layout operation",
		contract.WindowManagerCommandLayoutRedo,
		contract.WindowManagerRedoLayoutPayload{},
	)
}

func newLayoutHistoryCommand(
	deps commandDeps,
	use string,
	short string,
	commandID contract.WindowManagerCommandID,
	payload any,
) *cobra.Command {
	var flags windowManagerMutationFlags
	cmd := &cobra.Command{
		Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request, err := flags.request(cmd, commandID, payload)
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
	return cmd
}

func isWindowManagerArrangement(arrangement windowmanager.Arrangement) bool {
	switch arrangement {
	case windowmanager.ArrangementHorizontal, windowmanager.ArrangementVertical,
		windowmanager.ArrangementGrid, windowmanager.ArrangementStack:
		return true
	default:
		return false
	}
}
