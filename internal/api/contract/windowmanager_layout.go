package contract

import "github.com/compozy/compozy/internal/windowmanager"

// WindowManagerDesktop is one ordered workspace desktop in the stable wire contract.
type WindowManagerDesktop struct {
	ID             windowmanager.DesktopID      `json:"id"`
	Name           string                       `json:"name"`
	Order          int                          `json:"order"`
	Purpose        windowmanager.DesktopPurpose `json:"purpose"`
	FocusOwner     *windowmanager.WindowID      `json:"focus_owner,omitempty"`
	Groups         []WindowManagerLayoutGroup   `json:"groups"`
	Floating       []windowmanager.WindowID     `json:"floating"`
	FloatingStacks []WindowManagerFloatingStack `json:"floating_stacks"`
}

// WindowManagerFloatingStack is one ordered tab frame outside the tiled tree.
type WindowManagerFloatingStack struct {
	ID        windowmanager.NodeID         `json:"id"`
	WindowIDs []windowmanager.WindowID     `json:"window_ids"`
	ActiveID  *windowmanager.WindowID      `json:"active_id,omitempty"`
	Rect      windowmanager.NormalizedRect `json:"rect"`
	Minimized bool                         `json:"minimized"`
}

// WindowManagerLayoutGroup is one independent tiled island in a desktop.
type WindowManagerLayoutGroup struct {
	ID    windowmanager.GroupID        `json:"id"`
	Frame windowmanager.NormalizedRect `json:"frame"`
	Root  WindowManagerLayoutNode      `json:"root"`
}

// WindowManagerLayoutNode is a recursive leaf, weighted split, or tabbed stack.
type WindowManagerLayoutNode struct {
	ID        windowmanager.NodeID      `json:"id"`
	Kind      windowmanager.NodeKind    `json:"kind"`
	WindowID  *windowmanager.WindowID   `json:"window_id,omitempty"`
	Axis      *windowmanager.Axis       `json:"axis,omitempty"`
	Children  []WindowManagerLayoutNode `json:"children,omitempty"`
	Weights   []float64                 `json:"weights,omitempty"`
	WindowIDs []windowmanager.WindowID  `json:"window_ids,omitempty"`
	ActiveID  *windowmanager.WindowID   `json:"active_id,omitempty"`
}

func windowManagerDesktopsFromDomain(desktops []windowmanager.Desktop) []WindowManagerDesktop {
	result := make([]WindowManagerDesktop, 0, len(desktops))
	for _, desktop := range desktops {
		groups := make([]WindowManagerLayoutGroup, 0, len(desktop.Groups))
		for _, group := range desktop.Groups {
			groups = append(groups, windowManagerLayoutGroupFromDomain(group))
		}
		result = append(result, WindowManagerDesktop{
			ID: desktop.ID, Name: desktop.Name, Order: desktop.Order, Purpose: desktop.Purpose,
			FocusOwner: cloneWindowManagerPointer(desktop.FocusOwner), Groups: groups,
			Floating:       append([]windowmanager.WindowID{}, desktop.Floating...),
			FloatingStacks: windowManagerFloatingStacksFromDomain(desktop.FloatingStacks),
		})
	}
	return result
}

func windowManagerFloatingStacksFromDomain(stacks []windowmanager.FloatingStack) []WindowManagerFloatingStack {
	result := make([]WindowManagerFloatingStack, 0, len(stacks))
	for _, stack := range stacks {
		result = append(result, WindowManagerFloatingStack{
			ID: stack.ID, WindowIDs: append([]windowmanager.WindowID(nil), stack.WindowIDs...),
			ActiveID: cloneWindowManagerPointer(stack.ActiveID), Rect: stack.Rect, Minimized: stack.Minimized,
		})
	}
	return result
}

func windowManagerLayoutGroupFromDomain(group windowmanager.LayoutGroup) WindowManagerLayoutGroup {
	return WindowManagerLayoutGroup{
		ID: group.ID, Frame: group.Frame, Root: windowManagerLayoutNodeFromDomain(group.Root),
	}
}

func optionalWindowManagerLayoutGroupFromDomain(
	group *windowmanager.LayoutGroup,
) *WindowManagerLayoutGroup {
	if group == nil {
		return nil
	}
	converted := windowManagerLayoutGroupFromDomain(*group)
	return &converted
}

func windowManagerLayoutNodeFromDomain(node windowmanager.LayoutNode) WindowManagerLayoutNode {
	children := make([]WindowManagerLayoutNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, windowManagerLayoutNodeFromDomain(child))
	}
	return WindowManagerLayoutNode{
		ID: node.ID, Kind: node.Kind,
		WindowID: cloneWindowManagerPointer(node.WindowID), Axis: cloneWindowManagerPointer(node.Axis),
		Children: children, Weights: append([]float64(nil), node.Weights...),
		WindowIDs: append([]windowmanager.WindowID(nil), node.WindowIDs...),
		ActiveID:  cloneWindowManagerPointer(node.ActiveID),
	}
}

func windowManagerDesktopsToDomain(desktops []WindowManagerDesktop) []windowmanager.Desktop {
	result := make([]windowmanager.Desktop, 0, len(desktops))
	for _, desktop := range desktops {
		groups := make([]windowmanager.LayoutGroup, 0, len(desktop.Groups))
		for _, group := range desktop.Groups {
			groups = append(groups, windowManagerLayoutGroupToDomain(group))
		}
		result = append(result, windowmanager.Desktop{
			ID: desktop.ID, Name: desktop.Name, Order: desktop.Order, Purpose: desktop.Purpose,
			FocusOwner: cloneWindowManagerPointer(desktop.FocusOwner), Groups: groups,
			Floating:       append([]windowmanager.WindowID(nil), desktop.Floating...),
			FloatingStacks: windowManagerFloatingStacksToDomain(desktop.FloatingStacks),
		})
	}
	return result
}

func windowManagerFloatingStacksToDomain(stacks []WindowManagerFloatingStack) []windowmanager.FloatingStack {
	result := make([]windowmanager.FloatingStack, 0, len(stacks))
	for _, stack := range stacks {
		result = append(result, windowmanager.FloatingStack{
			ID: stack.ID, WindowIDs: append([]windowmanager.WindowID(nil), stack.WindowIDs...),
			ActiveID: cloneWindowManagerPointer(stack.ActiveID), Rect: stack.Rect, Minimized: stack.Minimized,
		})
	}
	return result
}

func windowManagerLayoutGroupToDomain(group WindowManagerLayoutGroup) windowmanager.LayoutGroup {
	return windowmanager.LayoutGroup{
		ID: group.ID, Frame: group.Frame, Root: windowManagerLayoutNodeToDomain(group.Root),
	}
}

func optionalWindowManagerLayoutGroupToDomain(
	group *WindowManagerLayoutGroup,
) *windowmanager.LayoutGroup {
	if group == nil {
		return nil
	}
	converted := windowManagerLayoutGroupToDomain(*group)
	return &converted
}

func windowManagerLayoutNodeToDomain(node WindowManagerLayoutNode) windowmanager.LayoutNode {
	children := make([]windowmanager.LayoutNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, windowManagerLayoutNodeToDomain(child))
	}
	return windowmanager.LayoutNode{
		ID: node.ID, Kind: node.Kind,
		WindowID: cloneWindowManagerPointer(node.WindowID), Axis: cloneWindowManagerPointer(node.Axis),
		Children: children, Weights: append([]float64(nil), node.Weights...),
		WindowIDs: append([]windowmanager.WindowID(nil), node.WindowIDs...),
		ActiveID:  cloneWindowManagerPointer(node.ActiveID),
	}
}

func cloneWindowManagerPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
