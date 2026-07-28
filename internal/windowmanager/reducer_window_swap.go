package windowmanager

import "fmt"

func (r *reducer) swapWindows(snapshot *Snapshot, command SwapWindowsCommand) (bool, error) {
	if command.FirstWindowID == command.SecondWindowID {
		return false, nil
	}
	first, firstExists := snapshot.Windows[command.FirstWindowID]
	second, secondExists := snapshot.Windows[command.SecondWindowID]
	if !firstExists || !secondExists {
		return false, fmt.Errorf("swap window is missing: %w", ErrWindowNotFound)
	}
	firstPlacement, firstFound := findWindowPlacement(snapshot, first.ID)
	secondPlacement, secondFound := findWindowPlacement(snapshot, second.ID)
	if !firstFound || !secondFound {
		return false, fmt.Errorf("swap placement is missing: %w", ErrInvalidTopology)
	}
	if err := swapMemberships(
		snapshot,
		firstPlacement,
		secondPlacement,
		command.FirstWindowID,
		command.SecondWindowID,
	); err != nil {
		return false, err
	}
	first.DesktopID = snapshot.Desktops[secondPlacement.desktopIndex].ID
	second.DesktopID = snapshot.Desktops[firstPlacement.desktopIndex].ID
	snapshot.Windows[first.ID] = first
	snapshot.Windows[second.ID] = second
	r.changes.window(first.ID)
	r.changes.window(second.ID)
	r.changes.desktop(first.DesktopID)
	r.changes.desktop(second.DesktopID)
	return true, nil
}

func swapMemberships(
	snapshot *Snapshot,
	firstPlacement windowPlacement,
	secondPlacement windowPlacement,
	firstID WindowID,
	secondID WindowID,
) error {
	if firstPlacement.desktopIndex == secondPlacement.desktopIndex &&
		firstPlacement.floatingIndex >= 0 && secondPlacement.floatingIndex >= 0 {
		floating := snapshot.Desktops[firstPlacement.desktopIndex].Floating
		floating[firstPlacement.floatingIndex], floating[secondPlacement.floatingIndex] =
			floating[secondPlacement.floatingIndex], floating[firstPlacement.floatingIndex]
		return nil
	}
	if firstPlacement.desktopIndex == secondPlacement.desktopIndex &&
		firstPlacement.groupIndex >= 0 && firstPlacement.groupIndex == secondPlacement.groupIndex {
		root := &snapshot.Desktops[firstPlacement.desktopIndex].Groups[firstPlacement.groupIndex].Root
		if swapped := swapNodeWindows(root, firstID, secondID); swapped != 2 {
			return fmt.Errorf("swap group membership count %d: %w", swapped, ErrInvalidTopology)
		}
		return nil
	}
	setMembershipAtPlacement(snapshot, firstPlacement, secondID, firstID)
	setMembershipAtPlacement(snapshot, secondPlacement, firstID, secondID)
	return nil
}

func setMembershipAtPlacement(snapshot *Snapshot, placement windowPlacement, newID, oldID WindowID) {
	desktop := &snapshot.Desktops[placement.desktopIndex]
	if placement.floatingIndex >= 0 {
		desktop.Floating[placement.floatingIndex] = newID
		return
	}
	replaceNodeWindow(&desktop.Groups[placement.groupIndex].Root, oldID, newID)
}

func replaceNodeWindow(node *LayoutNode, oldID, newID WindowID) bool {
	if node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == oldID {
		node.WindowID = &newID
		return true
	}
	if node.Kind == NodeKindStack {
		for index, id := range node.WindowIDs {
			if id == oldID {
				node.WindowIDs[index] = newID
				if node.ActiveID != nil && *node.ActiveID == oldID {
					node.ActiveID = &newID
				}
				return true
			}
		}
	}
	for index := range node.Children {
		if replaceNodeWindow(&node.Children[index], oldID, newID) {
			return true
		}
	}
	return false
}

func swapNodeWindows(node *LayoutNode, firstID, secondID WindowID) int {
	swapped := 0
	if node.Kind == NodeKindLeaf && node.WindowID != nil {
		switch *node.WindowID {
		case firstID:
			node.WindowID = &secondID
			swapped++
		case secondID:
			node.WindowID = &firstID
			swapped++
		}
	}
	if node.Kind == NodeKindStack {
		for index, windowID := range node.WindowIDs {
			switch windowID {
			case firstID:
				node.WindowIDs[index] = secondID
				swapped++
			case secondID:
				node.WindowIDs[index] = firstID
				swapped++
			}
		}
		if node.ActiveID != nil {
			switch *node.ActiveID {
			case firstID:
				node.ActiveID = &secondID
			case secondID:
				node.ActiveID = &firstID
			}
		}
	}
	for index := range node.Children {
		swapped += swapNodeWindows(&node.Children[index], firstID, secondID)
	}
	return swapped
}
