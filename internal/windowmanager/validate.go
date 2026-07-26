package windowmanager

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// ValidateSnapshot checks every committed aggregate invariant without repairing input.
func ValidateSnapshot(snapshot Snapshot) error {
	validator := snapshotValidator{
		snapshot:  snapshot,
		windowIDs: sortedWindowIDs(snapshot.Windows),
		desktops:  make(map[DesktopID]struct{}, len(snapshot.Desktops)),
		groups:    make(map[GroupID]struct{}), nodes: make(map[NodeID]struct{}),
		memberships: make(map[WindowID]int, len(snapshot.Windows)),
	}
	validator.validate()
	if len(validator.diagnostics) > 0 {
		return &TopologyError{Diagnostics: validator.diagnostics}
	}
	return nil
}

type snapshotValidator struct {
	snapshot    Snapshot
	windowIDs   []WindowID
	desktops    map[DesktopID]struct{}
	groups      map[GroupID]struct{}
	nodes       map[NodeID]struct{}
	memberships map[WindowID]int
	diagnostics []Diagnostic
}

func (v *snapshotValidator) validate() {
	v.validateHeader()
	v.validateDesktopIdentities()
	v.validateWindowRecords()
	for index := range v.snapshot.Desktops {
		v.validateDesktop(index)
	}
	v.validateMembershipCompleteness()
}

func (v *snapshotValidator) validateHeader() {
	if v.snapshot.Version != SnapshotVersion {
		v.add("topology.unsupported_version", "version", "document version must be 2")
	}
	if strings.TrimSpace(string(v.snapshot.WorkspaceID)) == "" {
		v.add("topology.workspace_required", "workspace_id", "workspace ID is required")
	}
	if len(v.snapshot.Desktops) == 0 {
		v.add("topology.final_desktop_required", "desktops", "at least one desktop is required")
	}
}

func (v *snapshotValidator) validateDesktopIdentities() {
	for index, desktop := range v.snapshot.Desktops {
		path := fmt.Sprintf("desktops[%d]", index)
		if strings.TrimSpace(string(desktop.ID)) == "" {
			v.add("topology.desktop_id_required", path+".id", "desktop ID is required")
		} else if _, duplicate := v.desktops[desktop.ID]; duplicate {
			v.add("topology.desktop_duplicate", path+".id", "desktop ID must be unique")
		}
		v.desktops[desktop.ID] = struct{}{}
		if desktop.Order != index {
			v.add("topology.desktop_order", path+".order", "desktop order must be contiguous")
		}
		if desktop.Purpose != DesktopPurposeStandard && desktop.Purpose != DesktopPurposeFocus {
			v.add("topology.desktop_purpose", path+".purpose", "desktop purpose is invalid")
		}
	}
}

func (v *snapshotValidator) validateWindowRecords() {
	for _, windowID := range v.windowIDs {
		window := v.snapshot.Windows[windowID]
		path := fmt.Sprintf("windows[%q]", windowID)
		if window.ID != windowID || strings.TrimSpace(string(windowID)) == "" {
			v.add("topology.window_identity", path+".id", "window map key and ID must match")
		}
		if strings.TrimSpace(window.App) == "" {
			v.add("topology.window_app_required", path+".app", "window app is required")
		}
		canonicalRoute, err := CanonicalRouteIntent(window.Route)
		if err != nil {
			v.add("topology.window_route", path+".route", err.Error())
		} else if !routeIntentsEqual(window.Route, canonicalRoute) {
			v.add("topology.window_route_canonical", path+".route", "window route must be canonical")
		}
		if _, exists := v.desktops[window.DesktopID]; !exists {
			v.add("topology.window_desktop_unknown", path+".desktop_id", "window desktop does not exist")
		}
		if !validRect(window.FloatingRect) {
			v.add("topology.floating_rect", path+".floating_rect", "floating rectangle must be normalized")
		}
		v.validateReturnAnchor(windowID, window.ReturnAnchor, path+".return_anchor")
	}
}

func (v *snapshotValidator) validateMembershipCompleteness() {
	for _, windowID := range v.windowIDs {
		window := v.snapshot.Windows[windowID]
		if v.memberships[windowID] != 1 {
			v.add("topology.window_membership", fmt.Sprintf("windows[%q]", windowID), "window must appear exactly once")
		}
		if window.Minimized && window.Placement != WindowPlacementFloating {
			v.add(
				"topology.minimized_placement",
				fmt.Sprintf("windows[%q].placement", windowID),
				"minimized windows must be floating",
			)
		}
	}
}

func (v *snapshotValidator) validateDesktop(index int) {
	desktop := v.snapshot.Desktops[index]
	path := fmt.Sprintf("desktops[%d]", index)
	for left := range desktop.Groups {
		group := desktop.Groups[left]
		groupPath := fmt.Sprintf("%s.groups[%d]", path, left)
		if strings.TrimSpace(string(group.ID)) == "" {
			v.add("topology.group_id_required", groupPath+".id", "group ID is required")
		} else if _, duplicate := v.groups[group.ID]; duplicate {
			v.add("topology.group_duplicate", groupPath+".id", "group ID must be unique")
		}
		v.groups[group.ID] = struct{}{}
		if !validRect(group.Frame) {
			v.add("topology.group_frame", groupPath+".frame", "group frame must be normalized")
		}
		for right := left + 1; right < len(desktop.Groups); right++ {
			if rectsOverlap(group.Frame, desktop.Groups[right].Frame) {
				v.add("topology.group_overlap", groupPath+".frame", "group frames must not overlap")
			}
		}
		v.validateNode(group.Root, desktop.ID, groupPath+".root")
	}
	for floatingIndex, windowID := range desktop.Floating {
		memberPath := fmt.Sprintf("%s.floating[%d]", path, floatingIndex)
		window, exists := v.snapshot.Windows[windowID]
		if !exists {
			v.add("topology.window_unknown", memberPath, "floating window does not exist")
			continue
		}
		v.memberships[windowID]++
		if window.DesktopID != desktop.ID {
			v.add("topology.window_cross_desktop", memberPath, "window belongs to another desktop")
		}
		if window.Placement != WindowPlacementFloating {
			v.add("topology.window_placement", memberPath, "floating membership and placement disagree")
		}
	}
	if desktop.Purpose == DesktopPurposeFocus {
		empty := len(desktop.Groups) == 0 && len(desktop.Floating) == 0
		if desktop.FocusOwner == nil && !empty {
			v.add("topology.focus_owner_required", path+".focus_owner", "focus desktop requires an owner")
		} else if desktop.FocusOwner == nil {
			return
		} else if owner, exists := v.snapshot.Windows[*desktop.FocusOwner]; !exists {
			v.add("topology.focus_owner_invalid", path+".focus_owner", "focus owner must exist in the workspace")
		} else if owner.DesktopID != desktop.ID {
			v.add("topology.focus_owner_desktop", path+".focus_owner", "focus owner must belong to the focus desktop")
		}
	} else if desktop.FocusOwner != nil {
		v.add("topology.focus_owner_standard", path+".focus_owner", "standard desktop cannot have a focus owner")
	}
}

func sortedWindowIDs(windows map[WindowID]Window) []WindowID {
	ids := make([]WindowID, 0, len(windows))
	for id := range windows {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (v *snapshotValidator) validateNode(node LayoutNode, desktopID DesktopID, path string) {
	v.validateNodeIdentity(node, path)
	switch node.Kind {
	case NodeKindLeaf:
		v.validateLeafNode(node, desktopID, path)
	case NodeKindStack:
		v.validateStackNode(node, desktopID, path)
	case NodeKindSplit:
		v.validateSplitNode(node, desktopID, path)
	default:
		v.add("topology.node_kind", path+".kind", "node kind is invalid")
	}
}

func (v *snapshotValidator) validateNodeIdentity(node LayoutNode, path string) {
	if strings.TrimSpace(string(node.ID)) == "" {
		v.add("topology.node_id_required", path+".id", "node ID is required")
	} else if _, duplicate := v.nodes[node.ID]; duplicate {
		v.add("topology.node_cycle_or_duplicate", path+".id", "node ID must occur once")
	}
	v.nodes[node.ID] = struct{}{}
}

func (v *snapshotValidator) validateLeafNode(node LayoutNode, desktopID DesktopID, path string) {
	if node.WindowID == nil {
		v.add("topology.leaf_window_required", path+".window_id", "leaf window is required")
		return
	}
	v.validateNodeWindow(*node.WindowID, desktopID, WindowPlacementTiled, path+".window_id")
}

func (v *snapshotValidator) validateStackNode(node LayoutNode, desktopID DesktopID, path string) {
	if len(node.WindowIDs) < 2 {
		v.add("topology.stack_size", path+".window_ids", "stack requires at least two windows")
	}
	for index, windowID := range node.WindowIDs {
		v.validateNodeWindow(
			windowID,
			desktopID,
			WindowPlacementStacked,
			fmt.Sprintf("%s.window_ids[%d]", path, index),
		)
	}
	if node.ActiveID == nil || !containsWindowID(node.WindowIDs, valueOrZero(node.ActiveID)) {
		v.add("topology.stack_active", path+".active_id", "stack active window must be a member")
	}
}

func (v *snapshotValidator) validateSplitNode(node LayoutNode, desktopID DesktopID, path string) {
	if node.Axis == nil || (*node.Axis != AxisHorizontal && *node.Axis != AxisVertical) {
		v.add("topology.split_axis", path+".axis", "split axis is invalid")
	}
	if len(node.Children) < 2 {
		v.add("topology.split_size", path+".children", "split requires at least two children")
	}
	if len(node.Children) != len(node.Weights) {
		v.add("topology.split_weight_count", path+".weights", "split child and weight counts must match")
	}
	total := 0.0
	for weightIndex, weight := range node.Weights {
		if !finite(weight) || weight <= 0 {
			v.add(
				"topology.split_weight",
				fmt.Sprintf("%s.weights[%d]", path, weightIndex),
				"split weights must be finite and positive",
			)
		}
		total += weight
	}
	if !finite(total) || math.Abs(total-1) > weightTolerance {
		v.add("topology.split_weight_sum", path+".weights", "split weights must sum to one")
	}
	for childIndex, child := range node.Children {
		v.validateSplitChild(node, child, desktopID, path, childIndex)
	}
}

func (v *snapshotValidator) validateSplitChild(
	parent LayoutNode,
	child LayoutNode,
	desktopID DesktopID,
	path string,
	childIndex int,
) {
	childPath := fmt.Sprintf("%s.children[%d]", path, childIndex)
	if child.Kind == NodeKindSplit && parent.Axis != nil && child.Axis != nil && *parent.Axis == *child.Axis {
		v.add("topology.split_not_flattened", childPath, "same-axis nested splits must be flattened")
	}
	v.validateNode(child, desktopID, childPath)
}

func (v *snapshotValidator) validateNodeWindow(
	windowID WindowID,
	desktopID DesktopID,
	placement WindowPlacement,
	path string,
) {
	window, exists := v.snapshot.Windows[windowID]
	if !exists {
		v.add("topology.window_unknown", path, "node window does not exist")
		return
	}
	v.memberships[windowID]++
	if window.DesktopID != desktopID {
		v.add("topology.window_cross_desktop", path, "window belongs to another desktop")
	}
	if window.Placement != placement {
		v.add("topology.window_placement", path, "node membership and placement disagree")
	}
}

func (v *snapshotValidator) add(code, path, message string) {
	v.diagnostics = append(v.diagnostics, Diagnostic{Code: code, Path: path, Message: message})
}

func validRect(rect NormalizedRect) bool {
	return finite(rect.X) && finite(rect.Y) && finite(rect.Width) && finite(rect.Height) &&
		rect.X >= 0 && rect.Y >= 0 && rect.Width > 0 && rect.Height > 0 &&
		rect.X+rect.Width <= 1+weightTolerance && rect.Y+rect.Height <= 1+weightTolerance
}

func rectsOverlap(left, right NormalizedRect) bool {
	if !validRect(left) || !validRect(right) {
		return false
	}
	return left.X < right.X+right.Width-weightTolerance && right.X < left.X+left.Width-weightTolerance &&
		left.Y < right.Y+right.Height-weightTolerance && right.Y < left.Y+left.Height-weightTolerance
}

func valueOrZero[T comparable](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
