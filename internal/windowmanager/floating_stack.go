package windowmanager

type stackLocation struct {
	desktopIndex       int
	groupIndex         int
	floatingStackIndex int
	node               *LayoutNode
	floating           *FloatingStack
}

func findStackByWindow(snapshot *Snapshot, windowID WindowID) (stackLocation, bool) {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return stackLocation{}, false
	}
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return stackLocation{}, false
	}
	desktop := &snapshot.Desktops[desktopIndex]
	for index := range desktop.FloatingStacks {
		stack := &desktop.FloatingStacks[index]
		if containsWindowID(stack.WindowIDs, windowID) {
			return stackLocation{
				desktopIndex:       desktopIndex,
				groupIndex:         -1,
				floatingStackIndex: index,
				floating:           stack,
			}, true
		}
	}
	for groupIndex := range desktop.Groups {
		if node, found := findStackNodeByWindow(&desktop.Groups[groupIndex].Root, windowID); found {
			return stackLocation{
				desktopIndex:       desktopIndex,
				groupIndex:         groupIndex,
				floatingStackIndex: -1,
				node:               node,
			}, true
		}
	}
	return stackLocation{}, false
}

func findStackByID(snapshot *Snapshot, stackID NodeID) (stackLocation, bool) {
	for desktopIndex := range snapshot.Desktops {
		desktop := &snapshot.Desktops[desktopIndex]
		for index := range desktop.FloatingStacks {
			if desktop.FloatingStacks[index].ID == stackID {
				return stackLocation{
					desktopIndex:       desktopIndex,
					groupIndex:         -1,
					floatingStackIndex: index,
					floating:           &desktop.FloatingStacks[index],
				}, true
			}
		}
		for groupIndex := range desktop.Groups {
			node, found := findNode(&desktop.Groups[groupIndex].Root, stackID)
			if found && node.Kind == NodeKindStack {
				return stackLocation{
					desktopIndex:       desktopIndex,
					groupIndex:         groupIndex,
					floatingStackIndex: -1,
					node:               node,
				}, true
			}
		}
	}
	return stackLocation{}, false
}

func findStackNodeByWindow(node *LayoutNode, windowID WindowID) (*LayoutNode, bool) {
	if node.Kind == NodeKindStack && containsWindowID(node.WindowIDs, windowID) {
		return node, true
	}
	for index := range node.Children {
		if found, exists := findStackNodeByWindow(&node.Children[index], windowID); exists {
			return found, true
		}
	}
	return nil, false
}

func (location stackLocation) id() NodeID {
	if location.floating != nil {
		return location.floating.ID
	}
	return location.node.ID
}

func (location stackLocation) members() []WindowID {
	if location.floating != nil {
		return location.floating.WindowIDs
	}
	return location.node.WindowIDs
}

func (location stackLocation) activeID() *WindowID {
	if location.floating != nil {
		return location.floating.ActiveID
	}
	return location.node.ActiveID
}

func (location stackLocation) setMembers(windowIDs []WindowID) {
	if location.floating != nil {
		location.floating.WindowIDs = windowIDs
		return
	}
	location.node.WindowIDs = windowIDs
}

func (location stackLocation) setActive(windowID WindowID) {
	if location.floating != nil {
		location.floating.ActiveID = &windowID
		return
	}
	location.node.ActiveID = &windowID
}

func (location stackLocation) rect(snapshot *Snapshot) NormalizedRect {
	if location.floating != nil {
		return location.floating.Rect
	}
	return snapshot.Desktops[location.desktopIndex].Groups[location.groupIndex].Frame
}

func collatePinned(windowIDs []WindowID, windows map[WindowID]Window) []WindowID {
	pinned := make([]WindowID, 0, len(windowIDs))
	unpinned := make([]WindowID, 0, len(windowIDs))
	for _, windowID := range windowIDs {
		if windows[windowID].Pinned {
			pinned = append(pinned, windowID)
		} else {
			unpinned = append(unpinned, windowID)
		}
	}
	return append(pinned, unpinned...)
}

func pinnedPrefixLength(windowIDs []WindowID, windows map[WindowID]Window) int {
	count := 0
	for _, windowID := range windowIDs {
		if !windows[windowID].Pinned {
			break
		}
		count++
	}
	return count
}

func nextActiveAfterRemoval(members []WindowID, removedIndex int) *WindowID {
	if len(members) == 0 {
		return nil
	}
	index := min(removedIndex, len(members)-1)
	active := members[index]
	return &active
}
