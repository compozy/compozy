package windowmanager

type CommandID string

const (
	CommandDesktopCreate        CommandID = "desktop.create"
	CommandDesktopUpdate        CommandID = "desktop.update"
	CommandDesktopReorder       CommandID = "desktop.reorder"
	CommandDesktopSwitch        CommandID = "desktop.switch"
	CommandDesktopDelete        CommandID = "desktop.delete"
	CommandWindowOpen           CommandID = "window.open"
	CommandWindowNavigate       CommandID = "window.navigate"
	CommandWindowClose          CommandID = "window.close"
	CommandWindowFocus          CommandID = "window.focus"
	CommandWindowMove           CommandID = "window.move"
	CommandWindowResize         CommandID = "window.resize"
	CommandWindowSwap           CommandID = "window.swap"
	CommandWindowToggleFloating CommandID = "window.toggle_floating"
	CommandWindowZoom           CommandID = "window.zoom"
	CommandWindowStackGroup     CommandID = "window.stack.group"
	CommandWindowStackReorder   CommandID = "window.stack.reorder"
	CommandWindowStackSetActive CommandID = "window.stack.set_active"
	CommandWindowPin            CommandID = "window.pin"
	CommandWindowReopen         CommandID = "window.reopen"
	CommandLayoutArrange        CommandID = "layout.arrange"
	CommandLayoutResize         CommandID = "layout.resize"
	CommandLayoutFrameResize    CommandID = "layout.frame_resize"
	CommandLayoutBalance        CommandID = "layout.balance"
	CommandLayoutUndo           CommandID = "layout.undo"
	CommandLayoutRedo           CommandID = "layout.redo"
	CommandLayoutReplace        CommandID = "layout.replace"
)

// Command is one typed semantic mutation payload.
type Command interface {
	CommandID() CommandID
}

// CommandRequest binds one semantic command to a workspace revision.
type CommandRequest struct {
	WorkspaceID      WorkspaceID
	CommandID        CommandID
	ExpectedRevision Revision
	// ClientID is required for desktop.switch and window.focus.
	ClientID *ClientID
	Actor    Actor
	Origin   string
	Rebase   *RebaseGuard
	Payload  Command
}

// RebaseGuard proves stale source and target identities are still unambiguous.
type RebaseGuard struct {
	WindowID      *WindowID
	SourceNodeID  *NodeID
	TargetNodeID  *NodeID
	SplitID       *NodeID
	BoundaryIndex *int
}

type CreateDesktopCommand struct {
	DesktopID DesktopID
	Name      string
	AfterID   *DesktopID
}

func (CreateDesktopCommand) CommandID() CommandID { return CommandDesktopCreate }

type UpdateDesktopCommand struct {
	DesktopID DesktopID
	Name      string
}

func (UpdateDesktopCommand) CommandID() CommandID { return CommandDesktopUpdate }

type ReorderDesktopCommand struct {
	DesktopID DesktopID
	Order     int
}

func (ReorderDesktopCommand) CommandID() CommandID { return CommandDesktopReorder }

type SwitchDesktopCommand struct{ DesktopID DesktopID }

func (SwitchDesktopCommand) CommandID() CommandID { return CommandDesktopSwitch }

type DeleteDesktopCommand struct {
	DesktopID     DesktopID
	DestinationID *DesktopID
}

func (DeleteDesktopCommand) CommandID() CommandID { return CommandDesktopDelete }

type WindowSpec struct {
	ID                  WindowID       `json:"id,omitempty"`
	App                 string         `json:"app"`
	InstanceKey         *string        `json:"instance_key,omitempty"`
	Route               RouteIntent    `json:"route"`
	DesktopID           DesktopID      `json:"desktop_id"`
	FloatingRect        NormalizedRect `json:"floating_rect"`
	InsertTiled         bool           `json:"insert_tiled,omitempty"`
	StackTargetWindowID *WindowID      `json:"stack_target_window_id,omitempty"`
}

type OpenWindowCommand struct {
	Window          WindowSpec
	RestoreWindowID *WindowID
}

func (OpenWindowCommand) CommandID() CommandID { return CommandWindowOpen }

type NavigateWindowCommand struct {
	WindowID WindowID    `json:"window_id"`
	Route    RouteIntent `json:"route,omitzero"`
	// InstanceKey retargets the window to another app instance in the same
	// navigation; valid only with replace mode and resets the nav stack.
	InstanceKey *string      `json:"instance_key,omitempty"`
	Mode        NavigateMode `json:"mode,omitempty"`
}

func (NavigateWindowCommand) CommandID() CommandID { return CommandWindowNavigate }

// CloseWindowCommand minimizes instead of deleting when Minimize is true.
type CloseWindowCommand struct {
	WindowID WindowID   `json:"window_id"`
	Minimize bool       `json:"minimize,omitempty"`
	Scope    CloseScope `json:"scope,omitempty"`
}

func (CloseWindowCommand) CommandID() CommandID { return CommandWindowClose }

type NavigateMode string

const (
	NavigateReplace NavigateMode = ""
	NavigatePush    NavigateMode = "push"
	NavigatePop     NavigateMode = "pop"
)

type CloseScope string

const (
	CloseScopeTab    CloseScope = ""
	CloseScopeGroup  CloseScope = "group"
	CloseScopeOthers CloseScope = "others"
	CloseScopeRight  CloseScope = "right"
)

type GroupWindowsCommand struct {
	TargetWindowID WindowID   `json:"target_window_id"`
	WindowIDs      []WindowID `json:"window_ids"`
	InsertIndex    *int       `json:"insert_index,omitempty"`
}

func (GroupWindowsCommand) CommandID() CommandID { return CommandWindowStackGroup }

type ReorderStackCommand struct {
	WindowID WindowID `json:"window_id"`
	Index    int      `json:"index"`
}

func (ReorderStackCommand) CommandID() CommandID { return CommandWindowStackReorder }

type SetStackActiveCommand struct {
	WindowID WindowID `json:"window_id"`
}

func (SetStackActiveCommand) CommandID() CommandID { return CommandWindowStackSetActive }

type PinWindowCommand struct {
	WindowID WindowID `json:"window_id"`
	Pinned   bool     `json:"pinned"`
}

func (PinWindowCommand) CommandID() CommandID { return CommandWindowPin }

type ReopenCommand struct{}

func (ReopenCommand) CommandID() CommandID { return CommandWindowReopen }

type FocusDirection string

const (
	FocusLeft  FocusDirection = "left"
	FocusRight FocusDirection = "right"
	FocusUp    FocusDirection = "up"
	FocusDown  FocusDirection = "down"
)

type FocusWindowCommand struct {
	WindowID  *WindowID
	Direction FocusDirection
}

func (FocusWindowCommand) CommandID() CommandID { return CommandWindowFocus }

type DropPlacement string

const (
	DropFloating DropPlacement = "floating"
	DropBefore   DropPlacement = "before"
	DropAfter    DropPlacement = "after"
	DropLeft     DropPlacement = "left"
	DropRight    DropPlacement = "right"
	DropTop      DropPlacement = "top"
	DropBottom   DropPlacement = "bottom"
	DropCenter   DropPlacement = "center"
)

type MoveWindowCommand struct {
	WindowID             WindowID
	DestinationDesktopID DesktopID
	TargetWindowID       *WindowID
	Placement            DropPlacement
	FloatingRect         *NormalizedRect
	MoveGroup            bool
}

func (MoveWindowCommand) CommandID() CommandID { return CommandWindowMove }

// ResizeWindowCommand assigns a normalized frame to the unit containing the
// window: a floating rect, a floating stack rect, a solo group frame, or a
// split member that detaches into its own island at the requested frame.
type ResizeWindowCommand struct {
	WindowID WindowID
	Frame    NormalizedRect
}

func (ResizeWindowCommand) CommandID() CommandID { return CommandWindowResize }

type SwapWindowsCommand struct {
	FirstWindowID  WindowID
	SecondWindowID WindowID
}

func (SwapWindowsCommand) CommandID() CommandID { return CommandWindowSwap }

type ToggleFloatingCommand struct {
	WindowID     WindowID
	FloatingRect *NormalizedRect
}

func (ToggleFloatingCommand) CommandID() CommandID { return CommandWindowToggleFloating }

type ZoomWindowCommand struct{ WindowID WindowID }

func (ZoomWindowCommand) CommandID() CommandID { return CommandWindowZoom }

type Arrangement string

const (
	ArrangementHorizontal Arrangement = "horizontal"
	ArrangementVertical   Arrangement = "vertical"
	ArrangementGrid       Arrangement = "grid"
	ArrangementStack      Arrangement = "stack"
)

type ArrangeLayoutCommand struct {
	DesktopID   DesktopID
	WindowIDs   []WindowID
	Arrangement Arrangement
	Frame       NormalizedRect
	GroupID     GroupID
	ResourceID  string
}

func (ArrangeLayoutCommand) CommandID() CommandID { return CommandLayoutArrange }

type ResizeLayoutCommand struct {
	SplitID       NodeID
	BoundaryIndex int
	Delta         float64
}

func (ResizeLayoutCommand) CommandID() CommandID { return CommandLayoutResize }

// GroupFrameEdit assigns one group frame inside an atomic multi-group resize.
type GroupFrameEdit struct {
	GroupID GroupID
	Frame   NormalizedRect
}

// FrameResizeLayoutCommand moves shared group boundaries by rewriting every
// affected island frame in one atomic, overlap-checked mutation.
type FrameResizeLayoutCommand struct {
	DesktopID DesktopID
	Edits     []GroupFrameEdit
}

func (FrameResizeLayoutCommand) CommandID() CommandID { return CommandLayoutFrameResize }

type BalanceLayoutCommand struct {
	GroupID *GroupID
	SplitID *NodeID
}

func (BalanceLayoutCommand) CommandID() CommandID { return CommandLayoutBalance }

type UndoLayoutCommand struct{}

func (UndoLayoutCommand) CommandID() CommandID { return CommandLayoutUndo }

type RedoLayoutCommand struct{}

func (RedoLayoutCommand) CommandID() CommandID { return CommandLayoutRedo }

type ReplaceLayoutCommand struct{ Document LayoutDocument }

func (ReplaceLayoutCommand) CommandID() CommandID { return CommandLayoutReplace }

// ChangeSet names the entities affected by one command.
type ChangeSet struct {
	DesktopIDs     []DesktopID `json:"desktop_ids,omitempty"`
	WindowIDs      []WindowID  `json:"window_ids,omitempty"`
	GroupIDs       []GroupID   `json:"group_ids,omitempty"`
	NodeIDs        []NodeID    `json:"node_ids,omitempty"`
	ClientIDs      []ClientID  `json:"client_ids,omitempty"`
	StackGrouped   []NodeID    `json:"stack_grouped,omitempty"`
	StackUngrouped []NodeID    `json:"stack_ungrouped,omitempty"`
}

// Result reports one applied or no-op command.
type Result struct {
	Snapshot    Snapshot     `json:"snapshot"`
	Applied     bool         `json:"applied"`
	Changes     ChangeSet    `json:"changes"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Client      *ClientView  `json:"client,omitempty"`
	RebasedFrom *Revision    `json:"rebased_from,omitempty"`
}

// Preview reports a validated proposal without a durable write.
type Preview struct {
	Snapshot    Snapshot     `json:"snapshot"`
	Changed     bool         `json:"changed"`
	Changes     ChangeSet    `json:"changes"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Client      *ClientView  `json:"client,omitempty"`
}
