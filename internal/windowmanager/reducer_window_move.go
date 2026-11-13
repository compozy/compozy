package windowmanager

import "fmt"

func (r *reducer) moveWindow(snapshot *Snapshot, c MoveWindowCommand) (bool, error) {
	w, ok := snapshot.Windows[c.WindowID]
	if !ok {
		return false, ErrWindowNotFound
	}
	di, ok := desktopIndexByID(snapshot, c.DestinationDesktopID)
	if !ok {
		return false, ErrDesktopNotFound
	}
	p, ok := findWindowPlacement(snapshot, w.ID)
	if !ok {
		return false, ErrInvalidTopology
	}
	source := snapshot.Desktops[p.desktopIndex].Tabs[p.tabIndex]
	targetID, err := resolveMoveTarget(snapshot, c, w.ID)
	if err != nil {
		return false, err
	}
	edge, err := validateMovePlacement(c.Placement, targetID)
	if err != nil {
		return false, err
	}
	whole := c.MoveGroup || len(nodeWindowIDs(source.Root)) == 1
	if whole && targetID == source.ID {
		return false, ErrInvalidCommand
	}
	if !edge && whole && w.DesktopID == c.DestinationDesktopID && targetID == "" {
		return false, nil
	}
	incoming := source.Root
	if whole {
		removeTabAt(&snapshot.Desktops[p.desktopIndex], p.tabIndex)
	} else {
		var err error
		incoming, err = newLeaf(w.ID, r.generate)
		if err != nil {
			return false, err
		}
		removeWindow(snapshot, w.ID)
	}
	d := &snapshot.Desktops[di]
	if edge {
		if err := r.mergeMovedTree(snapshot, d, targetID, incoming, c); err != nil {
			return false, err
		}
	} else {
		if err := r.placeMovedTab(snapshot, d, source, incoming, w, targetID, whole); err != nil {
			return false, err
		}
	}
	r.changes.tab(source.ID)
	for _, id := range nodeWindowIDs(incoming) {
		r.changes.window(id)
	}
	r.changes.desktop(w.DesktopID)
	r.changes.desktop(d.ID)
	return true, nil
}

func validateMovePlacement(placement DropPlacement, targetID TabID) (bool, error) {
	_, _, edge := placementAxis(placement)
	if !edge && placement != DropTab && placement != DropCenter {
		return false, fmt.Errorf("placement %q: %w", placement, ErrInvalidCommand)
	}
	if edge && targetID == "" {
		return false, ErrInvalidCommand
	}
	return edge, nil
}

func splitAtRoot(root, incoming LayoutNode, p DropPlacement, generate idGenerator) (LayoutNode, bool, error) {
	axis, before, ok := placementAxis(p)
	if !ok {
		return root, false, ErrInvalidCommand
	}
	if root.Kind == NodeKindSplit && root.Axis != nil && *root.Axis == axis {
		if before {
			root.Children = append([]LayoutNode{incoming}, root.Children...)
		} else {
			root.Children = append(root.Children, incoming)
		}
		root.Weights = equalWeights(len(root.Children))
		return flattenNode(root), true, nil
	}
	id, err := generate("node")
	if err != nil {
		return root, false, err
	}
	children := []LayoutNode{root, incoming}
	if before {
		children[0], children[1] = incoming, root
	}
	return flattenNode(
		LayoutNode{
			ID:       NodeID(id),
			Kind:     NodeKindSplit,
			Axis:     new(axis),
			Children: children,
			Weights:  []float64{.5, .5},
		},
	), true, nil
}

func resolveMoveTarget(snapshot *Snapshot, c MoveWindowCommand, windowID WindowID) (TabID, error) {
	var targetID TabID
	if c.TargetWindowID != nil {
		target, ok := snapshot.Windows[*c.TargetWindowID]
		if !ok {
			return "", ErrWindowNotFound
		}
		if target.ID == windowID || target.DesktopID != c.DestinationDesktopID {
			return "", ErrInvalidCommand
		}
		targetID = target.TabID
	}
	if c.TargetTabID != nil {
		if targetID != "" {
			return "", ErrInvalidCommand
		}
		d, _, ok := findTab(snapshot, *c.TargetTabID)
		if !ok || d.ID != c.DestinationDesktopID {
			return "", ErrInvalidCommand
		}
		targetID = *c.TargetTabID
	}
	return targetID, nil
}

func (r *reducer) mergeMovedTree(
	snapshot *Snapshot,
	d *Desktop,
	targetID TabID,
	incoming LayoutNode,
	c MoveWindowCommand,
) error {
	ti := tabHoldingID(d.Tabs, targetID)
	if ti < 0 {
		return ErrInvalidCommand
	}
	target := &d.Tabs[ti]
	var root LayoutNode
	var err error
	if c.TargetWindowID != nil {
		var found bool
		root, found, err = insertRelativeInNode(target.Root, *c.TargetWindowID, incoming, c.Placement, r.generate)
		if err == nil && !found {
			err = ErrWindowNotFound
		}
	} else {
		root, _, err = splitAtRoot(target.Root, incoming, c.Placement, r.generate)
	}
	if err != nil {
		return err
	}
	target.Root = root
	assignTabWindows(snapshot, d, *target)
	d.ActiveTabID = target.ID
	r.changes.tab(target.ID)
	r.changes.node(root.ID)
	return nil
}

func (r *reducer) placeMovedTab(
	snapshot *Snapshot,
	d *Desktop,
	source Tab,
	incoming LayoutNode,
	w Window,
	targetID TabID,
	whole bool,
) error {
	tab := source
	if !whole {
		var err error
		tab, err = r.newTab(w.ID)
		if err != nil {
			return err
		}
		tab.Root = incoming
		tab.Pinned = false
	}
	index := len(d.Tabs)
	if targetID != "" {
		index = tabHoldingID(d.Tabs, targetID) + 1
	} else if d.ID == w.DesktopID {
		index = tabHoldingID(d.Tabs, source.ID) + 1
	}
	insertTab(d, tab, index)
	assignTabWindows(snapshot, d, tab)
	d.ActiveTabID = tab.ID
	r.changes.tab(tab.ID)
	return nil
}
