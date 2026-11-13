package windowmanager

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

const (
	topologyUnsupportedVersionCode = "topology.unsupported_version"
	topologyWorkspaceMismatchCode  = "topology.workspace_mismatch"
)

// ValidateSnapshot checks every committed aggregate invariant without repairing input.
func ValidateSnapshot(snapshot Snapshot) error {
	validator := newSnapshotValidator(snapshot)
	validator.validate()
	validator.validateHistory()
	if len(validator.diagnostics) > 0 {
		return &TopologyError{Diagnostics: validator.diagnostics}
	}
	return nil
}

func newSnapshotValidator(snapshot Snapshot) snapshotValidator {
	return snapshotValidator{
		snapshot:  snapshot,
		windowIDs: sortedWindowIDs(snapshot.Windows),
		desktops:  make(map[DesktopID]struct{}, len(snapshot.Desktops)),
		tabs:      make(map[TabID]struct{}), nodes: make(map[NodeID]struct{}),
		memberships: make(map[WindowID]int, len(snapshot.Windows)),
	}
}

type snapshotValidator struct {
	snapshot    Snapshot
	windowIDs   []WindowID
	desktops    map[DesktopID]struct{}
	tabs        map[TabID]struct{}
	tabID       TabID
	nodes       map[NodeID]struct{}
	memberships map[WindowID]int
	diagnostics []Diagnostic
}

func (v *snapshotValidator) validate() {
	v.validateHeader()
	v.validateDesktopIdentities()
	v.validateWindowRecords()
	v.validateClosedEntries()
	for index := range v.snapshot.Desktops {
		v.validateDesktop(index)
	}
	v.validateMembershipCompleteness()
}

func (v *snapshotValidator) validateHeader() {
	if v.snapshot.Version != SnapshotVersion {
		v.add(
			topologyUnsupportedVersionCode,
			"version",
			fmt.Sprintf("document version must be %d", SnapshotVersion),
		)
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
		v.validateNavStack(window.NavStack, path+".nav_stack")
		if _, exists := v.desktops[window.DesktopID]; !exists {
			v.add("topology.window_desktop_unknown", path+".desktop_id", "window desktop does not exist")
		}
	}
}

func (v *snapshotValidator) validateMembershipCompleteness() {
	for _, windowID := range v.windowIDs {
		if v.memberships[windowID] != 1 {
			v.add("topology.window_membership", fmt.Sprintf("windows[%q]", windowID), "window must appear exactly once")
		}
	}
}

func (v *snapshotValidator) validateDesktop(index int) {
	desktop := v.snapshot.Desktops[index]
	path := fmt.Sprintf("desktops[%d]", index)
	if len(desktop.Tabs) == 0 {
		v.add("topology.tab_required", path+".tabs", "desktop requires at least one tab")
	}
	active := false
	regular := false
	for i, tab := range desktop.Tabs {
		tp := fmt.Sprintf("%s.tabs[%d]", path, i)
		if tab.ID == "" {
			v.add("topology.tab_id_required", tp+".id", "tab ID is required")
		}
		if _, duplicate := v.tabs[tab.ID]; duplicate {
			v.add("topology.tab_duplicate", tp+".id", "tab ID must be unique")
		}
		v.tabs[tab.ID] = struct{}{}
		active = active || desktop.ActiveTabID == tab.ID
		if tab.Pinned && regular {
			v.add("topology.tab_pinned_prefix", tp, "pinned tabs must form a contiguous prefix")
		}
		regular = regular || !tab.Pinned
		v.tabID = tab.ID
		v.validateNode(tab.Root, desktop.ID, tp+".root")
	}
	if !active {
		v.add("topology.active_tab", path+".active_tab_id", "active tab must belong to desktop")
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
	if node.Axis != nil || len(node.Children) != 0 || len(node.Weights) != 0 {
		v.add("topology.leaf_shape", path, "leaf cannot contain split fields")
	}
	if node.WindowID == nil {
		v.add("topology.leaf_window_required", path+".window_id", "leaf window is required")
		return
	}
	v.validateNodeWindow(*node.WindowID, desktopID, path+".window_id")
}

func (v *snapshotValidator) validateNavStack(routes []RouteIntent, path string) {
	if len(routes) > absoluteNavStackLimit {
		v.add("topology.nav_stack_limit", path, "navigation stack exceeds the absolute maximum")
	}
	for index, route := range routes {
		canonical, err := CanonicalRouteIntent(route)
		routePath := fmt.Sprintf("%s[%d]", path, index)
		if err != nil {
			v.add("topology.window_route", routePath, err.Error())
		} else if !routeIntentsEqual(route, canonical) {
			v.add("topology.window_route_canonical", routePath, "navigation route must be canonical")
		}
	}
}

func (v *snapshotValidator) validateClosedEntries() {
	if len(v.snapshot.ClosedEntries) > absoluteClosedEntryLimit {
		v.add("topology.closed_entry_limit", "closed_entries", "closed entries exceed the absolute maximum")
	}
	for entryIndex, entry := range v.snapshot.ClosedEntries {
		path := fmt.Sprintf("closed_entries[%d]", entryIndex)
		if len(entry.Windows) == 0 {
			v.add("topology.closed_entry_empty", path+".windows", "closed entry requires at least one window")
		}
		for windowIndex, window := range entry.Windows {
			windowPath := fmt.Sprintf("%s.windows[%d]", path, windowIndex)
			if strings.TrimSpace(string(window.ID)) == "" || strings.TrimSpace(window.App) == "" {
				v.add("topology.closed_window_identity", windowPath, "closed window identity and app are required")
			}
			canonical, err := CanonicalRouteIntent(window.Route)
			if err != nil {
				v.add("topology.window_route", windowPath+".route", err.Error())
			} else if !routeIntentsEqual(window.Route, canonical) {
				v.add("topology.window_route_canonical", windowPath+".route", "window route must be canonical")
			}
			v.validateNavStack(window.NavStack, windowPath+".nav_stack")
		}
		v.validateClosedTopology(entry, path)
	}
}

func (v *snapshotValidator) validateSplitNode(node LayoutNode, desktopID DesktopID, path string) {
	if node.WindowID != nil {
		v.add("topology.split_shape", path, "split cannot reference a window directly")
	}
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
	if window.TabID != v.tabID {
		v.add("topology.window_tab", path, "node membership and tab disagree")
	}
}

func (v *snapshotValidator) add(code, path, message string) {
	v.diagnostics = append(v.diagnostics, Diagnostic{Code: code, Path: path, Message: message})
}

func valueOrZero[T comparable](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
