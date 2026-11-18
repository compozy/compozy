package windowmanager

import (
	"encoding/json"
	"time"
)

const legacyNodeKindStack NodeKind = "stack"

// WindowPlacement is retained only for permanent v3/v4/v5 migration.
type WindowPlacement string

const (
	WindowPlacementTiled    WindowPlacement = "tiled"
	WindowPlacementStacked  WindowPlacement = "stacked"
	WindowPlacementFloating WindowPlacement = "floating"
)

type SnapshotV5 struct {
	Version       uint32                `json:"version"`
	WorkspaceID   WorkspaceID           `json:"workspace_id"`
	Revision      Revision              `json:"revision"`
	Desktops      []DesktopV5           `json:"desktops"`
	Windows       map[WindowID]WindowV5 `json:"windows"`
	ClosedEntries []ClosedEntryV5       `json:"closed_entries,omitempty"`
	History       json.RawMessage       `json:"history"`
	Overrides     WorkspaceConfigV5     `json:"overrides"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type DesktopV5 struct {
	ID             DesktopID         `json:"id"`
	Name           string            `json:"name"`
	Order          int               `json:"order"`
	Groups         []LayoutGroupV5   `json:"groups"`
	Floating       []WindowID        `json:"floating"`
	FloatingStacks []FloatingStackV5 `json:"floating_stacks"`
}

type FloatingStackV5 struct {
	ID        NodeID         `json:"id"`
	WindowIDs []WindowID     `json:"window_ids"`
	ActiveID  *WindowID      `json:"active_id,omitzero"`
	Rect      NormalizedRect `json:"rect"`
	Minimized bool           `json:"minimized"`
}

type LayoutGroupV5 struct {
	ID    GroupID        `json:"id"`
	Frame NormalizedRect `json:"frame"`
	Root  LayoutNodeV5   `json:"root"`
}

type LayoutNodeV5 struct {
	ID        NodeID         `json:"id"`
	Kind      NodeKind       `json:"kind"`
	WindowID  *WindowID      `json:"window_id,omitzero"`
	Axis      *Axis          `json:"axis,omitzero"`
	Children  []LayoutNodeV5 `json:"children,omitempty"`
	Weights   []float64      `json:"weights,omitempty"`
	WindowIDs []WindowID     `json:"window_ids,omitempty"`
	ActiveID  *WindowID      `json:"active_id,omitzero"`
}

type WindowV5 struct {
	ID           WindowID        `json:"id"`
	App          string          `json:"app"`
	InstanceKey  *string         `json:"instance_key,omitzero"`
	Route        RouteIntent     `json:"route"`
	NavStack     []RouteIntent   `json:"nav_stack"`
	Pinned       bool            `json:"pinned"`
	Placement    WindowPlacement `json:"placement"`
	DesktopID    DesktopID       `json:"desktop_id"`
	FloatingRect NormalizedRect  `json:"floating_rect"`
	Minimized    bool            `json:"minimized"`
	// Zoomed fills the desktop work area with the unit holding this window.
	Zoomed       bool            `json:"zoomed"`
	ReturnAnchor *ReturnAnchorV5 `json:"return_anchor,omitzero"`
}

type ClosedEntryV5 struct {
	Windows   []WindowV5     `json:"windows"`
	ActiveID  *WindowID      `json:"active_id,omitzero"`
	StackID   *NodeID        `json:"stack_id,omitzero"`
	DesktopID DesktopID      `json:"desktop_id"`
	Rect      NormalizedRect `json:"rect"`
	ClosedAt  time.Time      `json:"closed_at"`
}

type ReturnAnchorV5 struct {
	DesktopID      DesktopID        `json:"desktop_id"`
	GroupID        *GroupID         `json:"group_id,omitzero"`
	ParentSplitID  *NodeID          `json:"parent_split_id,omitzero"`
	ChildIndex     *int             `json:"child_index,omitzero"`
	Weight         *float64         `json:"weight,omitzero"`
	NeighborIDs    []WindowID       `json:"neighbor_ids,omitempty"`
	SourceRevision Revision         `json:"source_revision"`
	SourceGroup    *LayoutGroupV5   `json:"source_group,omitzero"`
	SourceStack    *FloatingStackV5 `json:"source_stack,omitzero"`
	// Zoomed restores the zoom a minimized window held when it left.
	Zoomed bool `json:"zoomed,omitzero"`
}

type WorkspaceConfigV5 struct {
	NewWindowPolicy     *NewWindowPolicy           `json:"new_window_policy,omitzero"`
	SmallViewportPolicy *SmallViewportPolicy       `json:"small_viewport_policy,omitzero"`
	FocusPolicy         *FocusPolicy               `json:"focus_policy,omitzero"`
	FocusWrap           *bool                      `json:"focus_wrap,omitzero"`
	FocusFollowsPointer *bool                      `json:"focus_follows_pointer,omitzero"`
	RaiseOnFocus        *bool                      `json:"raise_on_focus,omitzero"`
	DragAwayPolicy      *DragAwayPolicy            `json:"drag_away_policy,omitzero"`
	GroupMoveModifier   *string                    `json:"group_move_modifier,omitzero"`
	SwapModifier        *string                    `json:"swap_modifier,omitzero"`
	HistoryLimit        *int                       `json:"history_limit,omitzero"`
	NavStackLimit       *int                       `json:"nav_stack_limit,omitzero"`
	ClosedEntryLimit    *int                       `json:"closed_entry_limit,omitzero"`
	DesktopTransition   *DesktopTransition         `json:"desktop_transition,omitzero"`
	Gaps                *GapsConfig                `json:"gaps,omitzero"`
	Snap                *SnapConfig                `json:"snap,omitzero"`
	Bindings            *BindingsConfig            `json:"bindings,omitzero"`
	Shortcuts           map[string]ShortcutBinding `json:"shortcuts,omitempty"`
}

type LayoutDocumentV5 struct {
	Version     uint32                `json:"version"`
	WorkspaceID WorkspaceID           `json:"workspace_id"`
	Desktops    []DesktopV5           `json:"desktops"`
	Windows     map[WindowID]WindowV5 `json:"windows"`
	Overrides   WorkspaceConfigV5     `json:"overrides"`
}
