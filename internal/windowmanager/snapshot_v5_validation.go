package windowmanager

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

func validateLegacyTrees(snapshot SnapshotV5) error {
	for _, desktop := range snapshot.Desktops {
		for _, group := range desktop.Groups {
			if err := validateLegacyNode(group.Root); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLegacyNode(node LayoutNodeV5) error {
	invalid := func() error { return fmt.Errorf("legacy node %q is malformed: %w", node.ID, ErrInvalidTopology) }
	if node.ID == "" {
		return invalid()
	}
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID == nil || len(node.Children) != 0 {
			return invalid()
		}
	case legacyNodeKindStack:
		if len(node.WindowIDs) < 1 || node.ActiveID == nil || !slices.Contains(node.WindowIDs, *node.ActiveID) ||
			len(node.Children) != 0 {
			return invalid()
		}
	case NodeKindSplit:
		if !validLegacySplit(node) {
			return invalid()
		}
		for _, child := range node.Children {
			if err := validateLegacyNode(child); err != nil {
				return err
			}
		}
	default:
		return invalid()
	}
	return nil
}

func validLegacySplit(node LayoutNodeV5) bool {
	if node.Axis == nil || (*node.Axis != AxisHorizontal && *node.Axis != AxisVertical) ||
		len(node.Children) < 2 || len(node.Children) != len(node.Weights) {
		return false
	}
	sum := 0.0
	for _, weight := range node.Weights {
		if !finite(weight) || weight <= 0 {
			return false
		}
		sum += weight
	}
	return math.Abs(sum-1) <= weightTolerance
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
	maps.Copy(targets, resolved)
}
