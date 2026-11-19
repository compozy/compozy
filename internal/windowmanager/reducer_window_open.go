package windowmanager

import (
	"fmt"
	"strings"
)

func (r *reducer) openWindow(snapshot *Snapshot, command OpenWindowCommand) (bool, error) {
	spec := command.Window
	edge, err := openInsertEdge(spec)
	if err != nil {
		return false, err
	}
	if spec.StackTargetWindowID != nil {
		target, ok := snapshot.Windows[*spec.StackTargetWindowID]
		if !ok {
			return false, ErrWindowNotFound
		}
		if spec.DesktopID != "" && spec.DesktopID != target.DesktopID {
			return false, ErrInvalidCommand
		}
		spec.DesktopID = target.DesktopID
	}
	window, err := r.newOpenWindow(snapshot, spec)
	if err != nil {
		return false, err
	}
	di, _ := desktopIndexByID(snapshot, window.DesktopID)
	d := &snapshot.Desktops[di]
	anchor, active := r.openWindowAnchor(snapshot, d, spec)
	snapshot.Windows[window.ID] = window
	policy := r.config.NewWindowPolicy
	if command.policy != "" {
		policy = command.policy
	}
	split := spec.StackTargetWindowID == nil && (spec.InsertTiled || policy == NewWindowInsert)
	if split && anchor != nil {
		if err := r.insertOpenWindow(snapshot, d, window.ID, *anchor, active, edge); err != nil {
			return false, err
		}
	} else {
		tab, err := r.newTab(window.ID)
		if err != nil {
			return false, err
		}
		index := tabHoldingID(d.Tabs, active) + 1
		insertTab(d, tab, index)
		assignTabWindows(snapshot, d, tab)
		d.ActiveTabID = tab.ID
		r.changes.tab(tab.ID)
		r.changes.node(tab.Root.ID)
	}
	r.changes.window(window.ID)
	r.changes.desktop(d.ID)
	return true, nil
}

// openInsertEdge resolves where a tiled open lands beside its anchor. An edge
// is only meaningful for a tiled insert, so it is rejected without one.
func openInsertEdge(spec WindowSpec) (DropPlacement, error) {
	switch spec.InsertEdge {
	case "":
		return DropRight, nil
	case DropLeft, DropRight, DropTop, DropBottom:
	default:
		return "", fmt.Errorf(
			"insert_edge %q is not left, right, top or bottom: %w", spec.InsertEdge, ErrInvalidCommand,
		)
	}
	if !spec.InsertTiled || spec.StackTargetWindowID != nil {
		return "", fmt.Errorf("insert_edge requires insert_tiled without a stack target: %w", ErrInvalidCommand)
	}
	return spec.InsertEdge, nil
}

func (r *reducer) newTab(windowID WindowID) (Tab, error) {
	id, err := r.generate("tab")
	if err != nil {
		return Tab{}, fmt.Errorf("generate tab: %w", err)
	}
	leaf, err := newLeaf(windowID, r.generate)
	if err != nil {
		return Tab{}, err
	}
	return Tab{ID: TabID(id), Root: leaf}, nil
}

func (r *reducer) newOpenWindow(snapshot *Snapshot, spec WindowSpec) (Window, error) {
	id := spec.ID
	if id == "" {
		for range 128 {
			generated, err := r.generate("window")
			if err != nil {
				return Window{}, err
			}
			if !reservedWindowID(snapshot, WindowID(generated)) {
				id = WindowID(generated)
				break
			}
		}
	}
	if id == "" || reservedWindowID(snapshot, id) {
		return Window{}, fmt.Errorf("window %q already exists: %w", id, ErrInvalidCommand)
	}
	if strings.TrimSpace(spec.App) == "" {
		return Window{}, fmt.Errorf("window app is required: %w", ErrInvalidCommand)
	}
	desktop, err := r.resolveOpenDesktop(snapshot, spec.DesktopID)
	if err != nil {
		return Window{}, err
	}
	route, err := CanonicalRouteIntent(spec.Route)
	if err != nil {
		return Window{}, fmt.Errorf("window route: %w: %w", err, ErrInvalidCommand)
	}
	return Window{
		ID:          id,
		App:         strings.TrimSpace(spec.App),
		InstanceKey: clonePointer(spec.InstanceKey),
		DesktopID:   desktop,
		Route:       route,
		NavStack:    []RouteIntent{},
	}, nil
}

func (r *reducer) resolveOpenDesktop(snapshot *Snapshot, id DesktopID) (DesktopID, error) {
	if id != "" {
		if _, ok := desktopIndexByID(snapshot, id); !ok {
			return "", ErrDesktopNotFound
		}
		return id, nil
	}
	if r.focusedWindow != nil {
		if w, ok := snapshot.Windows[*r.focusedWindow]; ok {
			return w.DesktopID, nil
		}
	}
	if r.activeDesktop != nil {
		if _, ok := desktopIndexByID(snapshot, *r.activeDesktop); ok {
			return *r.activeDesktop, nil
		}
	}
	if len(snapshot.Desktops) == 0 {
		return "", ErrFinalDesktop
	}
	return snapshot.Desktops[0].ID, nil
}

func (r *reducer) openWindowAnchor(snapshot *Snapshot, d *Desktop, spec WindowSpec) (*WindowID, TabID) {
	anchor := spec.StackTargetWindowID
	if anchor == nil {
		anchor = spec.BesideWindowID
	}
	if anchor == nil {
		anchor = r.focusedWindow
	}
	active := d.ActiveTabID
	if r.activeTabs != nil {
		if chosen, ok := r.activeTabs[d.ID]; ok && tabHoldingID(d.Tabs, chosen) >= 0 {
			active = chosen
		}
	}
	if anchor != nil {
		if w, ok := snapshot.Windows[*anchor]; ok && w.DesktopID == d.ID {
			active = w.TabID
		} else {
			anchor = nil
		}
	}
	if anchor == nil {
		if ti := tabHoldingID(d.Tabs, active); ti >= 0 {
			ids := nodeWindowIDs(d.Tabs[ti].Root)
			if len(ids) > 0 {
				anchor = new(ids[0])
			}
		}
	}
	return anchor, active
}

func (r *reducer) insertOpenWindow(
	snapshot *Snapshot,
	d *Desktop,
	windowID WindowID,
	anchor WindowID,
	active TabID,
	edge DropPlacement,
) error {
	leaf, err := newLeaf(windowID, r.generate)
	if err != nil {
		return err
	}
	ti := tabHoldingID(d.Tabs, active)
	if ti < 0 {
		return ErrInvalidTopology
	}
	root, found, err := insertRelativeInNode(d.Tabs[ti].Root, anchor, leaf, edge, r.generate)
	if err != nil {
		return err
	}
	if !found {
		return ErrWindowNotFound
	}
	d.Tabs[ti].Root = root
	assignTabWindows(snapshot, d, d.Tabs[ti])
	r.changes.tab(active)
	r.changes.node(root.ID)
	return nil
}
