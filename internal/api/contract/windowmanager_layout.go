package contract

import "github.com/compozy/compozy/internal/windowmanager"

type WindowManagerDesktop struct {
	ID          windowmanager.DesktopID `json:"id"`
	Name        string                  `json:"name"`
	Order       int                     `json:"order"`
	Tabs        []WindowManagerTab      `json:"tabs"`
	ActiveTabID windowmanager.TabID     `json:"active_tab_id"`
}

type WindowManagerTab struct {
	ID     windowmanager.TabID     `json:"id"`
	Pinned bool                    `json:"pinned"`
	Root   WindowManagerLayoutNode `json:"root"`
}

type WindowManagerLayoutNode struct {
	ID       windowmanager.NodeID      `json:"id"`
	Kind     windowmanager.NodeKind    `json:"kind"`
	WindowID *windowmanager.WindowID   `json:"window_id,omitzero"`
	Axis     *windowmanager.Axis       `json:"axis,omitzero"`
	Children []WindowManagerLayoutNode `json:"children,omitempty"`
	Weights  []float64                 `json:"weights,omitempty"`
}

func windowManagerDesktopsFromDomain(desktops []windowmanager.Desktop) []WindowManagerDesktop {
	result := make([]WindowManagerDesktop, 0, len(desktops))
	for _, d := range desktops {
		tabs := make([]WindowManagerTab, 0, len(d.Tabs))
		for _, t := range d.Tabs {
			tabs = append(
				tabs,
				WindowManagerTab{ID: t.ID, Pinned: t.Pinned, Root: windowManagerLayoutNodeFromDomain(t.Root)},
			)
		}
		result = append(
			result,
			WindowManagerDesktop{ID: d.ID, Name: d.Name, Order: d.Order, Tabs: tabs, ActiveTabID: d.ActiveTabID},
		)
	}
	return result
}

func windowManagerDesktopsToDomain(desktops []WindowManagerDesktop) []windowmanager.Desktop {
	result := make([]windowmanager.Desktop, 0, len(desktops))
	for _, d := range desktops {
		tabs := make([]windowmanager.Tab, 0, len(d.Tabs))
		for _, t := range d.Tabs {
			tabs = append(
				tabs,
				windowmanager.Tab{ID: t.ID, Pinned: t.Pinned, Root: windowManagerLayoutNodeToDomain(t.Root)},
			)
		}
		result = append(
			result,
			windowmanager.Desktop{ID: d.ID, Name: d.Name, Order: d.Order, Tabs: tabs, ActiveTabID: d.ActiveTabID},
		)
	}
	return result
}

func windowManagerLayoutNodeFromDomain(n windowmanager.LayoutNode) WindowManagerLayoutNode {
	children := make([]WindowManagerLayoutNode, 0, len(n.Children))
	for _, c := range n.Children {
		children = append(children, windowManagerLayoutNodeFromDomain(c))
	}
	return WindowManagerLayoutNode{
		ID:       n.ID,
		Kind:     n.Kind,
		WindowID: cloneWindowManagerPointer(n.WindowID),
		Axis:     cloneWindowManagerPointer(n.Axis),
		Children: children,
		Weights:  append([]float64(nil), n.Weights...),
	}
}

func windowManagerLayoutNodeToDomain(n WindowManagerLayoutNode) windowmanager.LayoutNode {
	children := make([]windowmanager.LayoutNode, 0, len(n.Children))
	for _, c := range n.Children {
		children = append(children, windowManagerLayoutNodeToDomain(c))
	}
	return windowmanager.LayoutNode{
		ID:       n.ID,
		Kind:     n.Kind,
		WindowID: cloneWindowManagerPointer(n.WindowID),
		Axis:     cloneWindowManagerPointer(n.Axis),
		Children: children,
		Weights:  append([]float64(nil), n.Weights...),
	}
}

func cloneWindowManagerPointer[T any](v *T) *T {
	if v == nil {
		return nil
	}
	return new(*v)
}
