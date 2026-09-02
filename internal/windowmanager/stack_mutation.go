package windowmanager

import "fmt"

// detachStack removes the whole stack from its origin container and returns it
// as a stack node preserving identity, member order, and the active member.
func detachStack(snapshot *Snapshot, location stackLocation) (LayoutNode, bool) {
	desktop := &snapshot.Desktops[location.desktopIndex]
	if location.floatingStackIndex >= 0 {
		stack := desktop.FloatingStacks[location.floatingStackIndex]
		desktop.FloatingStacks = append(
			desktop.FloatingStacks[:location.floatingStackIndex],
			desktop.FloatingStacks[location.floatingStackIndex+1:]...)
		return LayoutNode{
			ID:        stack.ID,
			Kind:      NodeKindStack,
			WindowIDs: append([]WindowID(nil), stack.WindowIDs...),
			ActiveID:  clonePointer(stack.ActiveID),
		}, true
	}
	if location.node == nil {
		return LayoutNode{}, false
	}
	node := cloneNode(*location.node)
	root, removed := removeNodeFromTree(desktop.Groups[location.groupIndex].Root, node.ID)
	if !removed {
		return LayoutNode{}, false
	}
	if root.Kind == "" {
		desktop.Groups = append(
			desktop.Groups[:location.groupIndex],
			desktop.Groups[location.groupIndex+1:]...)
	} else {
		desktop.Groups[location.groupIndex].Root = root
	}
	return node, true
}

// removeNodeFromTree drops the whole subtree with the given id; an emptied
// root comes back with Kind "" exactly like removeWindowFromNode.
func removeNodeFromTree(node LayoutNode, nodeID NodeID) (LayoutNode, bool) {
	if node.ID == nodeID {
		return LayoutNode{}, true
	}
	if node.Kind != NodeKindSplit {
		return node, false
	}
	for index, child := range node.Children {
		updated, removed := removeNodeFromTree(child, nodeID)
		if !removed {
			continue
		}
		if updated.Kind == "" {
			node.Children = append(node.Children[:index], node.Children[index+1:]...)
			if index < len(node.Weights) {
				node.Weights = append(node.Weights[:index], node.Weights[index+1:]...)
			}
		} else {
			node.Children[index] = updated
		}
		return node, true
	}
	return node, false
}

// insertRelativeNode splices a prepared node beside the target's slot; center
// drops merge memberships instead and never reach this path.
func insertRelativeNode(
	snapshot *Snapshot,
	targetID WindowID,
	inserted LayoutNode,
	placement DropPlacement,
	generate idGenerator,
) error {
	if placement == DropCenter {
		return fmt.Errorf("node insert requires an axis placement: %w", ErrInvalidCommand)
	}
	targetPlacement, found := findWindowPlacement(snapshot, targetID)
	if !found || targetPlacement.groupIndex < 0 {
		return fmt.Errorf("target window %q is not tiled: %w", targetID, ErrInvalidCommand)
	}
	root := &snapshot.Desktops[targetPlacement.desktopIndex].Groups[targetPlacement.groupIndex].Root
	updated, insertedOK, err := insertRelativeInNode(*root, targetID, inserted, placement, generate)
	if err != nil {
		return err
	}
	if !insertedOK {
		return fmt.Errorf("target window %q: %w", targetID, ErrWindowNotFound)
	}
	*root = updated
	return nil
}
