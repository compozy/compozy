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
	firstStack, firstStacked := findStackByWindow(snapshot, command.FirstWindowID)
	secondStack, secondStacked := findStackByWindow(snapshot, command.SecondWindowID)
	if firstStacked && secondStacked && firstStack.id() == secondStack.id() {
		return false, nil
	}
	// A stacked window swaps as its whole tab frame: frames are the
	// arrangement unit, so the deck exchanges places with the target unit.
	if firstStacked || secondStacked {
		return r.swapUnits(snapshot, command.FirstWindowID, command.SecondWindowID)
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

type swapSlotKind int

const (
	swapSlotTree swapSlotKind = iota
	swapSlotFloatingWindow
	swapSlotFloatingStack
)

// swapUnit is one side of a unit swap: the arrangement unit the window belongs
// to (its whole tab frame when stacked, else the solo leaf or floating window)
// plus the slot it currently occupies.
type swapUnit struct {
	node         LayoutNode
	slotKind     swapSlotKind
	desktopIndex int
	slotNodeID   NodeID
	slotIndex    int
	slotRect     NormalizedRect
}

func captureSwapUnit(snapshot *Snapshot, windowID WindowID) (swapUnit, error) {
	if location, stacked := findStackByWindow(snapshot, windowID); stacked {
		if location.floating != nil {
			stack := *location.floating
			return swapUnit{
				node: LayoutNode{
					ID:        stack.ID,
					Kind:      NodeKindStack,
					WindowIDs: append([]WindowID(nil), stack.WindowIDs...),
					ActiveID:  clonePointer(stack.ActiveID),
				},
				slotKind:     swapSlotFloatingStack,
				desktopIndex: location.desktopIndex,
				slotIndex:    location.floatingStackIndex,
				slotRect:     stack.Rect,
			}, nil
		}
		return swapUnit{
			node:         cloneNode(*location.node),
			slotKind:     swapSlotTree,
			desktopIndex: location.desktopIndex,
			slotNodeID:   location.node.ID,
		}, nil
	}
	placement, found := findWindowPlacement(snapshot, windowID)
	if !found {
		return swapUnit{}, fmt.Errorf("swap placement is missing: %w", ErrInvalidTopology)
	}
	if placement.floatingIndex >= 0 {
		return swapUnit{
			node:         LayoutNode{Kind: NodeKindLeaf, WindowID: &windowID},
			slotKind:     swapSlotFloatingWindow,
			desktopIndex: placement.desktopIndex,
			slotIndex:    placement.floatingIndex,
			slotRect:     snapshot.Windows[windowID].FloatingRect,
		}, nil
	}
	if placement.groupIndex < 0 {
		return swapUnit{}, fmt.Errorf("swap placement is missing: %w", ErrInvalidTopology)
	}
	node, nodeFound := findNodeInSnapshot(snapshot, placement.nodeID)
	if !nodeFound || node.Kind != NodeKindLeaf {
		return swapUnit{}, fmt.Errorf("swap window %q node: %w", windowID, ErrInvalidTopology)
	}
	return swapUnit{
		node:         cloneNode(*node),
		slotKind:     swapSlotTree,
		desktopIndex: placement.desktopIndex,
		slotNodeID:   placement.nodeID,
	}, nil
}

// swapUnits exchanges the two windows' arrangement units in place: tree slots
// keep their tree position, floating slots keep their rect and z entry, and
// whichever unit arrives adopts the slot's shape.
func (r *reducer) swapUnits(snapshot *Snapshot, firstID, secondID WindowID) (bool, error) {
	firstUnit, err := captureSwapUnit(snapshot, firstID)
	if err != nil {
		return false, err
	}
	secondUnit, err := captureSwapUnit(snapshot, secondID)
	if err != nil {
		return false, err
	}
	// Both tree pointers resolve before either write so the crosswise
	// assignment never chases a slot the first write already renamed.
	firstSlot, err := resolveSwapTreeSlot(snapshot, firstUnit)
	if err != nil {
		return false, err
	}
	secondSlot, err := resolveSwapTreeSlot(snapshot, secondUnit)
	if err != nil {
		return false, err
	}
	if err := r.writeSwapUnit(snapshot, secondUnit.node, firstUnit, firstSlot); err != nil {
		return false, err
	}
	if err := r.writeSwapUnit(snapshot, firstUnit.node, secondUnit, secondSlot); err != nil {
		return false, err
	}
	r.retargetSwappedWindows(snapshot, secondUnit.node, firstUnit)
	r.retargetSwappedWindows(snapshot, firstUnit.node, secondUnit)
	if firstUnit.node.ID != "" {
		r.changes.node(firstUnit.node.ID)
	}
	if secondUnit.node.ID != "" {
		r.changes.node(secondUnit.node.ID)
	}
	r.changes.desktop(snapshot.Desktops[firstUnit.desktopIndex].ID)
	r.changes.desktop(snapshot.Desktops[secondUnit.desktopIndex].ID)
	return true, nil
}

func resolveSwapTreeSlot(snapshot *Snapshot, unit swapUnit) (*LayoutNode, error) {
	if unit.slotKind != swapSlotTree {
		return nil, nil
	}
	slot, found := findNodeInSnapshot(snapshot, unit.slotNodeID)
	if !found {
		return nil, fmt.Errorf("swap slot %q: %w", unit.slotNodeID, ErrInvalidTopology)
	}
	return slot, nil
}

func (r *reducer) writeSwapUnit(
	snapshot *Snapshot,
	incoming LayoutNode,
	slot swapUnit,
	treeSlot *LayoutNode,
) error {
	desktop := &snapshot.Desktops[slot.desktopIndex]
	switch slot.slotKind {
	case swapSlotTree:
		if incoming.Kind == NodeKindLeaf && incoming.ID == "" {
			nodeIDRaw, err := r.generate("node")
			if err != nil {
				return fmt.Errorf("generate swap leaf ID: %w", err)
			}
			incoming.ID = NodeID(nodeIDRaw)
		}
		*treeSlot = incoming
	case swapSlotFloatingWindow:
		if incoming.Kind == NodeKindLeaf {
			desktop.Floating[slot.slotIndex] = *incoming.WindowID
			return nil
		}
		desktop.Floating = append(
			desktop.Floating[:slot.slotIndex],
			desktop.Floating[slot.slotIndex+1:]...)
		desktop.FloatingStacks = append(desktop.FloatingStacks, FloatingStack{
			ID:        incoming.ID,
			WindowIDs: incoming.WindowIDs,
			ActiveID:  incoming.ActiveID,
			Rect:      slot.slotRect,
		})
	case swapSlotFloatingStack:
		if incoming.Kind == NodeKindStack {
			desktop.FloatingStacks[slot.slotIndex] = FloatingStack{
				ID:        incoming.ID,
				WindowIDs: incoming.WindowIDs,
				ActiveID:  incoming.ActiveID,
				Rect:      slot.slotRect,
			}
			return nil
		}
		desktop.FloatingStacks = append(
			desktop.FloatingStacks[:slot.slotIndex],
			desktop.FloatingStacks[slot.slotIndex+1:]...)
		desktop.Floating = append(desktop.Floating, *incoming.WindowID)
	}
	return nil
}

// retargetSwappedWindows re-homes every window of the unit that landed in the
// slot: desktop follows the slot, and placement adopts the slot's shape.
func (r *reducer) retargetSwappedWindows(snapshot *Snapshot, unit LayoutNode, slot swapUnit) {
	desktopID := snapshot.Desktops[slot.desktopIndex].ID
	if unit.Kind == NodeKindLeaf {
		window := snapshot.Windows[*unit.WindowID]
		window.DesktopID = desktopID
		if slot.slotKind == swapSlotTree {
			window.Placement = WindowPlacementTiled
		} else {
			window.Placement = WindowPlacementFloating
			window.FloatingRect = slot.slotRect
		}
		snapshot.Windows[*unit.WindowID] = window
		r.changes.window(*unit.WindowID)
		return
	}
	for _, memberID := range unit.WindowIDs {
		member := snapshot.Windows[memberID]
		member.DesktopID = desktopID
		member.Placement = WindowPlacementStacked
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
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

// replaceNodeWindow and swapNodeWindows only walk leaves: stacked members
// never reach the solo swap path — their whole frame swaps via swapUnits.
func replaceNodeWindow(node *LayoutNode, oldID, newID WindowID) bool {
	if node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == oldID {
		node.WindowID = &newID
		return true
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
	for index := range node.Children {
		swapped += swapNodeWindows(&node.Children[index], firstID, secondID)
	}
	return swapped
}
