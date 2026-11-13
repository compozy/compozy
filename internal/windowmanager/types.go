package windowmanager

import (
	"context"
	"encoding/json"
	"time"
)

const SnapshotVersion uint32 = 6

const PreviousSnapshotVersion uint32 = 4

const (
	absoluteNavStackLimit    = 200
	absoluteClosedEntryLimit = 100
)

type WorkspaceID string

// GlobalDesktopWorkspaceID is the reserved profile-owned desktop partition used
// when the operator has no project workspace. It never grants workspace data scope.
const GlobalDesktopWorkspaceID WorkspaceID = "global"

// UserDesktopWorkspaceID is the reserved User desktop partition within one profile.
const UserDesktopWorkspaceID WorkspaceID = "user"

// IsReservedDesktopPartition identifies partitions that do not require a project registration.
func IsReservedDesktopPartition(id WorkspaceID) bool {
	return id == GlobalDesktopWorkspaceID || id == UserDesktopWorkspaceID
}

type DesktopID string
type WindowID string
type NodeID string
type GroupID string
type TabID string
type ClientID string
type Revision uint64

type ClientKind string

const (
	ClientKindShell   ClientKind = "shell"
	ClientKindBrowser ClientKind = "browser"
)

type NodeKind string

const (
	NodeKindLeaf  NodeKind = "leaf"
	NodeKindSplit NodeKind = "split"
)

type Axis string

const (
	AxisHorizontal Axis = "horizontal"
	AxisVertical   Axis = "vertical"
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
	Version       uint32              `json:"version"`
	WorkspaceID   WorkspaceID         `json:"workspace_id"`
	Revision      Revision            `json:"revision"`
	Desktops      []Desktop           `json:"desktops"`
	Windows       map[WindowID]Window `json:"windows"`
	ClosedEntries []ClosedEntry       `json:"closed_entries,omitempty"`
	History       History             `json:"history"`
	Overrides     WorkspaceConfig     `json:"overrides"`
	Migration     *MigrationStatus    `json:"migration,omitzero"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// Desktop owns an ordered tab bar.
type Desktop struct {
	ID          DesktopID `json:"id"`
	Name        string    `json:"name"`
	Order       int       `json:"order"`
	Tabs        []Tab     `json:"tabs"`
	ActiveTabID TabID     `json:"active_tab_id"`
}

type Tab struct {
	ID     TabID      `json:"id"`
	Pinned bool       `json:"pinned"`
	Root   LayoutNode `json:"root"`
}

type LayoutNode struct {
	ID       NodeID       `json:"id"`
	Kind     NodeKind     `json:"kind"`
	WindowID *WindowID    `json:"window_id,omitzero"`
	Axis     *Axis        `json:"axis,omitzero"`
	Children []LayoutNode `json:"children,omitempty"`
	Weights  []float64    `json:"weights,omitempty"`
}

type Window struct {
	ID          WindowID      `json:"id"`
	App         string        `json:"app"`
	InstanceKey *string       `json:"instance_key,omitzero"`
	Route       RouteIntent   `json:"route"`
	NavStack    []RouteIntent `json:"nav_stack"`
	DesktopID   DesktopID     `json:"desktop_id"`
	TabID       TabID         `json:"tab_id"`
}

type ClosedEntry struct {
	Windows   []Window      `json:"windows"`
	Tabs      []ClosedTab   `json:"tabs,omitempty"`
	Member    *ClosedMember `json:"member,omitzero"`
	DesktopID DesktopID     `json:"desktop_id"`
	ClosedAt  time.Time     `json:"closed_at"`
}

type ClosedTab struct {
	ID     TabID      `json:"id"`
	Pinned bool       `json:"pinned"`
	Root   LayoutNode `json:"root"`
	Index  int        `json:"index"`
}

type Edge string

type ClosedMember struct {
	TabID         TabID    `json:"tab_id"`
	ParentSplitID NodeID   `json:"parent_split_id"`
	ChildIndex    int      `json:"child_index"`
	Weight        float64  `json:"weight"`
	NeighborID    WindowID `json:"neighbor_id"`
	NeighborEdge  Edge     `json:"neighbor_edge"`
}

// ClientView is transient presentation state for one connected client.
type ClientView struct {
	WorkspaceID          WorkspaceID                  `json:"workspace_id"`
	ClientID             ClientID                     `json:"client_id"`
	Kind                 ClientKind                   `json:"kind"`
	PresentationRevision uint64                       `json:"presentation_revision"`
	ContextRevision      uint64                       `json:"context_revision"`
	ActiveDesktopID      DesktopID                    `json:"active_desktop_id"`
	FocusedWindowID      *WindowID                    `json:"focused_window_id,omitzero"`
	FocusOrder           []WindowID                   `json:"focus_order"`
	ActiveTab            map[DesktopID]TabID          `json:"active_tab"`
	StackActive          map[NodeID]WindowID          `json:"stack_active"               deprecated:"true"`
	PaletteContext       PaletteContext               `json:"palette_context"`
	GlobalShortcuts      []GlobalShortcutRegistration `json:"global_shortcuts"`
	ConnectedAt          time.Time                    `json:"connected_at"`
	AttachmentToken      string                       `json:"attachment_token,omitempty"`
}

// PaletteContext is the volatile, client-targeted availability snapshot.
type PaletteContext struct {
	WindowFocused       bool         `json:"window_focused"`
	WindowFloating      bool         `json:"window_floating"                 deprecated:"true"`
	WindowInSplit       bool         `json:"window_in_split"`
	WindowStacked       bool         `json:"window_stacked"                  deprecated:"true"`
	DesktopWindowCount  int          `json:"desktop_window_count"`
	ScopeGlobal         bool         `json:"scope_global"`
	ShellDesktop        bool         `json:"shell_desktop"`
	FocusedSessionState string       `json:"focused_session_state,omitempty"`
	WorkspaceTrusted    bool         `json:"workspace_trusted"`
	DestinationIntent   *RouteIntent `json:"destination_intent,omitzero"`
}

// ClientRegistration requests a live client view.
type ClientRegistration struct {
	WorkspaceID     WorkspaceID
	ClientID        ClientID
	Kind            ClientKind
	ActiveDesktopID DesktopID
	Context         ClientContextInput
}

// ClientContextInput carries only client-owned volatile palette state.
type ClientContextInput struct {
	ScopeGlobal         bool
	FocusedSessionState string
	WorkspaceTrusted    bool
	DestinationIntent   *RouteIntent
	GlobalShortcuts     []GlobalShortcutRegistration
}

// ClientContextUpdate replaces one attached client's palette and global-shortcut context.
type ClientContextUpdate struct {
	WorkspaceID WorkspaceID
	ClientID    ClientID
	Context     ClientContextInput
}

// ClientCommand is one invocation the daemon dispatches to an attached client.
type ClientCommand struct {
	CommandID string          `json:"command_id"`
	Op        string          `json:"op"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// ClientCommandResponseStatus is the ack, result, or error stage of one command.
type ClientCommandResponseStatus string

const (
	ClientCommandAcknowledged ClientCommandResponseStatus = "ack"
	ClientCommandCompleted    ClientCommandResponseStatus = "result"
	ClientCommandFailed       ClientCommandResponseStatus = "error"
)

// ClientCommandResponse is the client's ack, result, or error for one command.
type ClientCommandResponse struct {
	CommandID string                      `json:"command_id"`
	Status    ClientCommandResponseStatus `json:"status"`
	Result    json.RawMessage             `json:"result,omitempty"`
	Error     string                      `json:"error,omitempty"`
}

// ClientCommandConnection is the live channel used to dispatch and resolve commands.
type ClientCommandConnection interface {
	Commands() <-chan ClientCommand
	Done() <-chan struct{}
	Err() error
	Resolve(ClientCommandResponse) error
	Close() error
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
	// DeleteWorkspace is atomic: an error leaves the workspace state unchanged.
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

// SubscriptionUpdate carries one ordered snapshot, topology, or client presentation change.
type SubscriptionUpdate struct {
	Snapshot *Snapshot
	Event    *Event
	Client   *ClientView
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
	Migration(context.Context, WorkspaceID, string, bool) (MigrationStatus, error)
	Snapshot(context.Context, WorkspaceID) (Snapshot, error)
	Preview(context.Context, CommandRequest) (Preview, error)
	Execute(context.Context, CommandRequest) (Result, error)
	Clients(context.Context, WorkspaceID) ([]ClientView, error)
	RegisterClient(context.Context, ClientRegistration) (ClientView, error)
	UpdateClientContext(context.Context, ClientContextUpdate) (ClientView, error)
	AuthorizeClient(context.Context, WorkspaceID, ClientID, string) error
	AttachClientCommands(context.Context, WorkspaceID, ClientID) (ClientCommandConnection, error)
	DispatchClientCommand(context.Context, WorkspaceID, ClientID, ClientCommand) (ClientCommandResponse, error)
	UnregisterClient(context.Context, WorkspaceID, ClientID) error
	ExportLayout(context.Context, WorkspaceID) (LayoutDocument, error)
	ValidateLayout(context.Context, WorkspaceID, LayoutDocument) (Validation, error)
	ReplaceLayout(context.Context, ReplaceLayoutRequest) (Result, error)
	LayoutResources(context.Context, WorkspaceID) ([]LayoutResource, error)
	Subscribe(context.Context, SubscriptionRequest) (Subscription, error)
}
