package windowmanager

import "fmt"

func (r *reducer) groupWindows(snapshot *Snapshot, command GroupWindowsCommand) (bool, error) {
	if len(command.WindowIDs) == 0 {
		return false, fmt.Errorf("windows to group are required: %w", ErrInvalidCommand)
	}
	target, exists := snapshot.Windows[command.TargetWindowID]
	if !exists {
		return false, fmt.Errorf("target window %q: %w", command.TargetWindowID, ErrWindowNotFound)
	}
	seen := make(map[WindowID]struct{}, len(command.WindowIDs))
	for _, windowID := range command.WindowIDs {
		if windowID == command.TargetWindowID {
			return false, fmt.Errorf("window cannot group with itself: %w", ErrInvalidCommand)
		}
		if _, duplicate := seen[windowID]; duplicate {
			return false, fmt.Errorf("window %q is duplicated: %w", windowID, ErrInvalidCommand)
		}
		seen[windowID] = struct{}{}
		if _, exists := snapshot.Windows[windowID]; !exists {
			return false, fmt.Errorf("window %q: %w", windowID, ErrWindowNotFound)
		}
	}
	targetStackID := NodeID("")
	if location, stacked := findStackByWindow(snapshot, command.TargetWindowID); stacked {
		targetStackID = location.id()
	}

	for _, windowID := range command.WindowIDs {
		sourceStackID := NodeID("")
		if location, stacked := findStackByWindow(snapshot, windowID); stacked {
			sourceStackID = location.id()
		}
		window := snapshot.Windows[windowID]
		if sourceStackID != targetStackID || targetStackID == "" {
			if !removeWindow(snapshot, windowID) {
				return false, fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
			}
			if sourceStackID != "" {
				if _, stillStacked := findStackByID(snapshot, sourceStackID); !stillStacked {
					r.changes.stackUngrouped(sourceStackID)
				}
			}
		}
		window.DesktopID = target.DesktopID
		window.Placement = WindowPlacementStacked
		window.Minimized = false
		window.Zoomed = false
		window.ReturnAnchor = nil
		snapshot.Windows[windowID] = window
		r.changes.window(windowID)
	}

	location, err := r.ensureTargetStack(snapshot, command.TargetWindowID)
	if err != nil {
		return false, err
	}
	existing := make([]WindowID, 0, len(location.members()))
	for _, windowID := range location.members() {
		if _, joining := seen[windowID]; !joining {
			existing = append(existing, windowID)
		}
	}
	members := spliceStackMembers(existing, command.WindowIDs, command.InsertIndex, snapshot.Windows)
	location.setMembers(members)
	location.setActive(command.WindowIDs[len(command.WindowIDs)-1])
	for _, memberID := range members {
		member := snapshot.Windows[memberID]
		member.DesktopID = target.DesktopID
		member.Placement = WindowPlacementStacked
		member.Minimized = false
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.desktop(target.DesktopID)
	r.changes.stackGrouped(location.id())
	return true, nil
}

func (r *reducer) ensureTargetStack(snapshot *Snapshot, targetID WindowID) (stackLocation, error) {
	if location, found := findStackByWindow(snapshot, targetID); found {
		return location, nil
	}
	placement, found := findWindowPlacement(snapshot, targetID)
	if !found {
		return stackLocation{}, fmt.Errorf("target window %q has no placement: %w", targetID, ErrInvalidTopology)
	}
	stackIDRaw, err := r.generate("node")
	if err != nil {
		return stackLocation{}, fmt.Errorf("generate stack ID: %w", err)
	}
	stackID := NodeID(stackIDRaw)
	target := snapshot.Windows[targetID]
	if placement.floatingIndex >= 0 {
		desktop := &snapshot.Desktops[placement.desktopIndex]
		desktop.Floating = append(
			desktop.Floating[:placement.floatingIndex],
			desktop.Floating[placement.floatingIndex+1:]...)
		target.Placement = WindowPlacementStacked
		target.Minimized = false
		snapshot.Windows[targetID] = target
		desktop.FloatingStacks = append(desktop.FloatingStacks, FloatingStack{
			ID: stackID, WindowIDs: []WindowID{targetID}, ActiveID: &targetID, Rect: target.FloatingRect,
		})
		index := len(desktop.FloatingStacks) - 1
		return stackLocation{
			desktopIndex:       placement.desktopIndex,
			groupIndex:         -1,
			floatingStackIndex: index,
			floating:           &desktop.FloatingStacks[index],
		}, nil
	}
	if placement.groupIndex < 0 {
		return stackLocation{}, fmt.Errorf("target window %q placement: %w", targetID, ErrInvalidTopology)
	}
	node, found := findNodeInSnapshot(snapshot, placement.nodeID)
	if !found || node.Kind != NodeKindLeaf {
		return stackLocation{}, fmt.Errorf("target window %q node: %w", targetID, ErrInvalidTopology)
	}
	*node = LayoutNode{ID: stackID, Kind: NodeKindStack, WindowIDs: []WindowID{targetID}, ActiveID: &targetID}
	target.Placement = WindowPlacementStacked
	snapshot.Windows[targetID] = target
	return stackLocation{
		desktopIndex:       placement.desktopIndex,
		groupIndex:         placement.groupIndex,
		floatingStackIndex: -1,
		node:               node,
	}, nil
}

func spliceStackMembers(
	existing []WindowID,
	joiners []WindowID,
	requested *int,
	windows map[WindowID]Window,
) []WindowID {
	pinnedJoiners := make([]WindowID, 0, len(joiners))
	unpinnedJoiners := make([]WindowID, 0, len(joiners))
	for _, windowID := range joiners {
		if windows[windowID].Pinned {
			pinnedJoiners = append(pinnedJoiners, windowID)
		} else {
			unpinnedJoiners = append(unpinnedJoiners, windowID)
		}
	}
	base := len(existing)
	if requested != nil {
		base = min(max(*requested, 0), len(existing))
	}
	pinnedCount := pinnedPrefixLength(existing, windows)
	pinnedIndex := min(base, pinnedCount)
	result := spliceWindowIDs(existing, pinnedIndex, pinnedJoiners)
	unpinnedBase := base
	if pinnedIndex <= base {
		unpinnedBase += len(pinnedJoiners)
	}
	unpinnedIndex := min(max(unpinnedBase, pinnedCount+len(pinnedJoiners)), len(result))
	return spliceWindowIDs(result, unpinnedIndex, unpinnedJoiners)
}

func spliceWindowIDs(values []WindowID, index int, inserted []WindowID) []WindowID {
	if len(inserted) == 0 {
		return append([]WindowID(nil), values...)
	}
	result := make([]WindowID, 0, len(values)+len(inserted))
	result = append(result, values[:index]...)
	result = append(result, inserted...)
	result = append(result, values[index:]...)
	return result
}
