package windowmanager

import (
	"context"
	"encoding/json"
	"time"
)

const SnapshotVersion uint32 = 2

type WorkspaceID string
type DesktopID string
type WindowID string
type NodeID string
type GroupID string
type ClientID string
type Revision uint64

type DesktopPurpose string

const (
	DesktopPurposeStandard DesktopPurpose = "standard"
	DesktopPurposeFocus    DesktopPurpose = "focus"
)

type NodeKind string

const (
	NodeKindLeaf  NodeKind = "leaf"
	NodeKindSplit NodeKind = "split"
	NodeKindStack NodeKind = "stack"
)

type Axis string

const (
	AxisHorizontal Axis = "horizontal"
	AxisVertical   Axis = "vertical"
)

type WindowPlacement string

const (
	WindowPlacementTiled    WindowPlacement = "tiled"
	WindowPlacementStacked  WindowPlacement = "stacked"
	WindowPlacementFloating WindowPlacement = "floating"
)

// RouteSearch is a canonical JSON object containing durable route search state.
type RouteSearch map[string]json.RawMessage

// RouteIntent is one application-internal durable navigation target.
type RouteIntent struct {
	Pathname string      `json:"pathname"`
	Search   RouteSearch `json:"search"`
}

// NormalizedRect is viewport-independent geometry in the closed unit square.
type NormalizedRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Snapshot is the complete durable workspace aggregate.
type Snapshot struct {
	Version     uint32              `json:"version"`
	WorkspaceID WorkspaceID         `json:"workspace_id"`
	Revision    Revision            `json:"revision"`
	Desktops    []Desktop           `json:"desktops"`
	Windows     map[WindowID]Window `json:"windows"`
	History     History             `json:"history"`
	Overrides   WorkspaceConfig     `json:"overrides"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// Desktop owns tiled groups and floating order.
type Desktop struct {
	ID         DesktopID      `json:"id"`
	Name       string         `json:"name"`
	Order      int            `json:"order"`
	Purpose    DesktopPurpose `json:"purpose"`
	FocusOwner *WindowID      `json:"focus_owner,omitempty"`
	Groups     []LayoutGroup  `json:"groups"`
	Floating   []WindowID     `json:"floating"`
}

// LayoutGroup is one independent tiled island.
type LayoutGroup struct {
	ID    GroupID        `json:"id"`
	Frame NormalizedRect `json:"frame"`
	Root  LayoutNode     `json:"root"`
}

// LayoutNode is a leaf, weighted split, or tabbed stack.
type LayoutNode struct {
	ID        NodeID       `json:"id"`
	Kind      NodeKind     `json:"kind"`
	WindowID  *WindowID    `json:"window_id,omitempty"`
	Axis      *Axis        `json:"axis,omitempty"`
	Children  []LayoutNode `json:"children,omitempty"`
	Weights   []float64    `json:"weights,omitempty"`
	WindowIDs []WindowID   `json:"window_ids,omitempty"`
	ActiveID  *WindowID    `json:"active_id,omitempty"`
}

// Window is one durable application instance.
type Window struct {
	ID           WindowID        `json:"id"`
	App          string          `json:"app"`
	InstanceKey  *string         `json:"instance_key,omitempty"`
	Route        RouteIntent     `json:"route"`
	Placement    WindowPlacement `json:"placement"`
	DesktopID    DesktopID       `json:"desktop_id"`
	FloatingRect NormalizedRect  `json:"floating_rect"`
	Minimized    bool            `json:"minimized"`
	ReturnAnchor *ReturnAnchor   `json:"return_anchor,omitempty"`
}

// ReturnAnchor describes a window's prior structural slot.
type ReturnAnchor struct {
	DesktopID      DesktopID    `json:"desktop_id"`
	GroupID        *GroupID     `json:"group_id,omitempty"`
	ParentSplitID  *NodeID      `json:"parent_split_id,omitempty"`
	ChildIndex     *int         `json:"child_index,omitempty"`
	Weight         *float64     `json:"weight,omitempty"`
	NeighborIDs    []WindowID   `json:"neighbor_ids,omitempty"`
	SourceRevision Revision     `json:"source_revision"`
	SourceGroup    *LayoutGroup `json:"source_group,omitempty"`
}

// ClientView is transient presentation state for one connected client.
type ClientView struct {
	WorkspaceID          WorkspaceID `json:"workspace_id"`
	ClientID             ClientID    `json:"client_id"`
	PresentationRevision uint64      `json:"presentation_revision"`
	ActiveDesktopID      DesktopID   `json:"active_desktop_id"`
	FocusedWindowID      *WindowID   `json:"focused_window_id,omitempty"`
	FocusOrder           []WindowID  `json:"focus_order"`
	ConnectedAt          time.Time   `json:"connected_at"`
}

// ClientRegistration requests a live client view.
type ClientRegistration struct {
	WorkspaceID     WorkspaceID
	ClientID        ClientID
	ActiveDesktopID DesktopID
}

// Actor records the initiator without affecting command semantics.
type Actor struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

// Event is published after one durable commit.
type Event struct {
	WorkspaceID WorkspaceID `json:"workspace_id"`
	Revision    Revision    `json:"revision"`
	CommandID   CommandID   `json:"command_id"`
	Changes     ChangeSet   `json:"changes"`
	Actor       Actor       `json:"actor"`
	Origin      string      `json:"origin,omitempty"`
	OccurredAt  time.Time   `json:"occurred_at"`
}

// Commit is the repository's atomic compare-and-swap unit.
type Commit struct {
	WorkspaceID      WorkspaceID
	ExpectedRevision Revision
	Snapshot         Snapshot
	Event            Event
}

// Repository stores the complete aggregate atomically.
type Repository interface {
	Load(ctx context.Context, workspaceID WorkspaceID) (Snapshot, error)
	Commit(ctx context.Context, commit *Commit) error
	DeleteWorkspace(ctx context.Context, workspaceID WorkspaceID) error
}

// WorkspaceResolver authorizes and resolves a workspace before state access.
type WorkspaceResolver interface {
	ResolveWorkspace(ctx context.Context, workspaceID WorkspaceID) error
}

// LayoutResourceRegistry resolves validated declarative layout resources.
type LayoutResourceRegistry interface {
	Resolve(ctx context.Context, workspaceID WorkspaceID, resourceID string) (LayoutDocument, error)
	List(ctx context.Context, workspaceID WorkspaceID) ([]LayoutResource, error)
}

// SubscriptionRequest binds an optional client view to one stream.
type SubscriptionRequest struct {
	WorkspaceID   WorkspaceID
	AfterRevision Revision
	ClientID      *ClientID
}

// SubscriptionFence atomically captures durable topology and optional presentation state.
type SubscriptionFence struct {
	Snapshot Snapshot
	Client   *ClientView
}

// SubscriptionUpdate carries one ordered topology or client presentation change.
type SubscriptionUpdate struct {
	Event  *Event
	Client *ClientView
}

// Subscription fences current state and later ordered updates.
type Subscription interface {
	Fence() SubscriptionFence
	Updates() <-chan SubscriptionUpdate
	Err() error
	Close() error
}

// Service is the window-manager runtime contract.
type Service interface {
	Snapshot(context.Context, WorkspaceID) (Snapshot, error)
	Preview(context.Context, CommandRequest) (Preview, error)
	Execute(context.Context, CommandRequest) (Result, error)
	Clients(context.Context, WorkspaceID) ([]ClientView, error)
	RegisterClient(context.Context, ClientRegistration) (ClientView, error)
	UnregisterClient(context.Context, WorkspaceID, ClientID) error
	ExportLayout(context.Context, WorkspaceID) (LayoutDocument, error)
	ValidateLayout(context.Context, WorkspaceID, LayoutDocument) (Validation, error)
	ReplaceLayout(context.Context, ReplaceLayoutRequest) (Result, error)
	LayoutResources(context.Context, WorkspaceID) ([]LayoutResource, error)
	Subscribe(context.Context, SubscriptionRequest) (Subscription, error)
}
