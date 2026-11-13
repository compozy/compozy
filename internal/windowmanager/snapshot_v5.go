package windowmanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
)

func decodeSnapshotJSON(encoded []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing snapshot data")
	}
	return nil
}

// DecodeSnapshot permanently upgrades stored formats without changing current snapshots.
func DecodeSnapshot(encoded []byte) (Snapshot, MigrationSummary, error) {
	var header struct {
		Version uint32 `json:"version"`
	}
	if err := json.Unmarshal(encoded, &header); err != nil {
		return Snapshot{}, MigrationSummary{}, err
	}
	if header.Version == SnapshotVersion {
		var current Snapshot
		if err := decodeSnapshotJSON(encoded, &current); err != nil {
			return Snapshot{}, MigrationSummary{}, err
		}
		return current, MigrationSummary{}, ValidateSnapshot(current)
	}
	var legacy SnapshotV5
	var err error
	switch header.Version {
	case 3:
		legacy, err = MigrateLegacySnapshotV3(encoded)
	case 4:
		legacy, err = MigrateSnapshotV4(encoded)
	case 5:
		err = decodeSnapshotJSON(encoded, &legacy)
	default:
		err = fmt.Errorf("snapshot version %d is unsupported: %w", header.Version, ErrInvalidTopology)
	}
	if err != nil {
		return Snapshot{}, MigrationSummary{}, err
	}
	return ConvertSnapshotV5(legacy)
}

type snapshotConverter struct {
	used    map[string]bool
	summary MigrationSummary
	windows map[WindowID]WindowV5
}

func (c *snapshotConverter) allocate(base string) string {
	candidate := base
	for suffix := 2; c.used[candidate]; suffix++ {
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
	c.used[candidate] = true
	return candidate
}

func (c *snapshotConverter) leaf(id WindowID) LayoutNode {
	return LayoutNode{ID: NodeID(c.allocate("leaf-" + string(id))), Kind: NodeKindLeaf, WindowID: new(id)}
}

func (c *snapshotConverter) solo(id WindowID) Tab {
	return Tab{ID: TabID(c.allocate("tab-" + string(id))), Pinned: c.windows[id].Pinned, Root: c.leaf(id)}
}

// ConvertSnapshotV5 preserves every content instance and rejects ambiguous membership.
func ConvertSnapshotV5(legacy SnapshotV5) (Snapshot, MigrationSummary, error) {
	if err := validateLegacyTrees(legacy); err != nil {
		return Snapshot{}, MigrationSummary{}, err
	}
	c := newSnapshotConverter(legacy)
	out := Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: legacy.WorkspaceID,
		Revision:    legacy.Revision,
		Windows:     map[WindowID]Window{},
		History:     History{Undo: []HistoryEntry{}, Redo: []HistoryEntry{}},
		UpdatedAt:   legacy.UpdatedAt,
	}
	if err := convertLegacyOverrides(legacy.Overrides, &out.Overrides); err != nil {
		return Snapshot{}, c.summary, err
	}
	if err := c.convertDesktops(legacy, &out); err != nil {
		return Snapshot{}, c.summary, err
	}
	for id := range legacy.Windows {
		if _, ok := out.Windows[id]; !ok {
			return Snapshot{}, c.summary, fmt.Errorf("legacy window %q has no node: %w", id, ErrInvalidTopology)
		}
	}
	c.convertClosedEntries(legacy.ClosedEntries, &out)
	c.summary.Desktops = len(out.Desktops)
	c.summary.Windows = len(out.Windows)
	for _, d := range out.Desktops {
		c.summary.Tabs += len(d.Tabs)
	}
	if err := ValidateSnapshot(out); err != nil {
		return Snapshot{}, c.summary, err
	}
	return out, c.summary, nil
}

func newSnapshotConverter(legacy SnapshotV5) snapshotConverter {
	c := snapshotConverter{used: map[string]bool{}, windows: maps.Clone(legacy.Windows)}
	for id := range legacy.Windows {
		c.used[string(id)] = true
	}
	for _, entry := range legacy.ClosedEntries {
		for _, w := range entry.Windows {
			c.used[string(w.ID)] = true
		}
	}
	for _, d := range legacy.Desktops {
		for _, group := range d.Groups {
			c.used[string(group.ID)] = true
			c.reserveNodes(group.Root)
		}
		for _, stack := range d.FloatingStacks {
			c.used[string(stack.ID)] = true
		}
	}
	return c
}

func convertLegacyOverrides(legacy WorkspaceConfigV5, current *WorkspaceConfig) error {
	// Retired overrides are translated only at the legacy boundary.
	encodedOverrides, err := json.Marshal(legacy)
	if err != nil {
		return fmt.Errorf("encode legacy overrides: %w", err)
	}
	if err := json.Unmarshal(encodedOverrides, current); err != nil {
		return err
	}
	current.RaiseOnFocus = nil
	current.DragAwayPolicy = nil
	current.GroupMoveModifier = nil
	current.Snap = nil
	current.Bindings = nil
	if current.NewWindowPolicy != nil && *current.NewWindowPolicy == NewWindowFloating {
		current.NewWindowPolicy = new(NewWindowTab)
	}
	return nil
}

func (c *snapshotConverter) convertClosedEntries(entries []ClosedEntryV5, out *Snapshot) {
	for _, entry := range entries {
		converted := ClosedEntry{DesktopID: entry.DesktopID, ClosedAt: entry.ClosedAt}
		for _, old := range entry.Windows {
			tab := c.solo(old.ID)
			tab.Pinned = old.Pinned
			index := 0
			if di, ok := desktopIndexByID(out, entry.DesktopID); ok {
				index = len(out.Desktops[di].Tabs)
			}
			converted.Tabs = append(
				converted.Tabs,
				ClosedTab{ID: tab.ID, Pinned: tab.Pinned, Root: tab.Root, Index: index + len(converted.Tabs)},
			)
			converted.Windows = append(converted.Windows, currentWindow(old, entry.DesktopID, tab.ID))
		}
		out.ClosedEntries = append(out.ClosedEntries, converted)
	}
}

func currentWindow(old WindowV5, desktopID DesktopID, tabID TabID) Window {
	return rewriteRetiredWindow(
		Window{
			ID:          old.ID,
			App:         old.App,
			InstanceKey: clonePointer(old.InstanceKey),
			Route:       cloneRouteIntent(old.Route),
			NavStack:    cloneRouteIntents(old.NavStack),
			DesktopID:   desktopID,
			TabID:       tabID,
		},
	)
}

func (c *snapshotConverter) reserveNodes(node LayoutNodeV5) {
	c.used[string(node.ID)] = true
	for _, child := range node.Children {
		c.reserveNodes(child)
	}
}

func (c *snapshotConverter) rootTabs(id TabID, root LayoutNodeV5) []Tab {
	if root.Kind == "stack" {
		tabs := make([]Tab, 0, len(root.WindowIDs))
		for i, w := range root.WindowIDs {
			var tab Tab
			if i == 0 {
				tab = Tab{ID: id, Root: LayoutNode{ID: root.ID, Kind: NodeKindLeaf, WindowID: new(w)}}
			} else {
				tab = c.solo(w)
			}
			tabs = append(tabs, tab)
		}
		return tabs
	}
	return c.treeTabs(id, root)
}

func (c *snapshotConverter) treeTabs(id TabID, root LayoutNodeV5) []Tab {
	extras := []WindowID{}
	var convert func(LayoutNodeV5) LayoutNode
	convert = func(old LayoutNodeV5) LayoutNode {
		n := LayoutNode{
			ID:       old.ID,
			Kind:     old.Kind,
			WindowID: clonePointer(old.WindowID),
			Axis:     clonePointer(old.Axis),
			Weights:  slices.Clone(old.Weights),
		}
		if old.Kind == "stack" {
			active := WindowID("")
			if len(old.WindowIDs) > 0 {
				active = old.WindowIDs[0]
			}
			if old.ActiveID != nil && slices.Contains(old.WindowIDs, *old.ActiveID) {
				active = *old.ActiveID
			}
			n = LayoutNode{ID: old.ID, Kind: NodeKindLeaf, WindowID: new(active)}
			for _, id := range old.WindowIDs {
				if id != active {
					extras = append(extras, id)
				}
			}
		}
		for _, child := range old.Children {
			n.Children = append(n.Children, convert(child))
		}
		return flattenNode(n)
	}
	tabs := []Tab{{ID: id, Root: convert(root)}}
	for _, w := range extras {
		tabs = append(tabs, c.solo(w))
	}
	return tabs
}

func tabHolding(tabs []Tab, id WindowID) TabID {
	for _, t := range tabs {
		if slices.Contains(nodeWindowIDs(t.Root), id) {
			return t.ID
		}
	}
	return ""
}

func legacyZoomTarget(d DesktopV5, s SnapshotV5) (WindowID, DesktopID, bool) {
	units := len(d.Groups) + len(d.FloatingStacks)
	for _, id := range d.Floating {
		if !s.Windows[id].Minimized {
			units++
		}
	}
	if units != 1 {
		return "", "", false
	}
	ids := make([]WindowID, 0)
	for _, g := range d.Groups {
		ids = append(ids, legacyNodeWindows(g.Root)...)
	}
	for _, st := range d.FloatingStacks {
		ids = append(ids, st.WindowIDs...)
	}
	ids = append(ids, d.Floating...)
	for _, id := range ids {
		w := s.Windows[id]
		if !w.Zoomed || w.ReturnAnchor == nil || w.ReturnAnchor.DesktopID == d.ID {
			continue
		}
		for _, target := range s.Desktops {
			if target.ID == w.ReturnAnchor.DesktopID {
				return id, target.ID, true
			}
		}
	}
	return "", "", false
}

func legacyNodeWindows(n LayoutNodeV5) []WindowID {
	if n.Kind == "stack" {
		return slices.Clone(n.WindowIDs)
	}
	if n.Kind == NodeKindLeaf && n.WindowID != nil {
		return []WindowID{*n.WindowID}
	}
	var ids []WindowID
	for _, child := range n.Children {
		ids = append(ids, legacyNodeWindows(child)...)
	}
	return ids
}
