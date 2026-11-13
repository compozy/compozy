package windowmanager

import (
	"fmt"
	"math"
	"slices"
)

func validateLegacyTrees(snapshot SnapshotV5) error {
	var validate func(LayoutNodeV5) error
	validate = func(node LayoutNodeV5) error {
		invalid := func() error { return fmt.Errorf("legacy node %q is malformed: %w", node.ID, ErrInvalidTopology) }
		if node.ID == "" {
			return invalid()
		}
		switch node.Kind {
		case NodeKindLeaf:
			if node.WindowID == nil || len(node.Children) != 0 {
				return invalid()
			}
		case "stack":
			if len(node.WindowIDs) < 1 || node.ActiveID == nil || !slices.Contains(node.WindowIDs, *node.ActiveID) ||
				len(node.Children) != 0 {
				return invalid()
			}
		case NodeKindSplit:
			if node.Axis == nil || (*node.Axis != AxisHorizontal && *node.Axis != AxisVertical) ||
				len(node.Children) < 2 ||
				len(node.Children) != len(node.Weights) {
				return invalid()
			}
			sum := 0.0
			for _, weight := range node.Weights {
				if !finite(weight) || weight <= 0 {
					return invalid()
				}
				sum += weight
			}
			if math.Abs(sum-1) > weightTolerance {
				return invalid()
			}
			for _, child := range node.Children {
				if err := validate(child); err != nil {
					return err
				}
			}
		default:
			return invalid()
		}
		return nil
	}
	for _, desktop := range snapshot.Desktops {
		for _, group := range desktop.Groups {
			if err := validate(group.Root); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveZoomTargets(targets map[DesktopID]DesktopID) {
	resolved := map[DesktopID]DesktopID{}
	for source, first := range targets {
		seen := map[DesktopID]bool{source: true}
		target := first
		for !seen[target] {
			seen[target] = true
			next, exists := targets[target]
			if !exists {
				resolved[source] = target
				break
			}
			target = next
		}
	}
	clear(targets)
	for source, target := range resolved {
		targets[source] = target
	}
}
