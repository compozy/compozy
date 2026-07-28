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
	CommandWindowSwap           CommandID = "window.swap"
	CommandWindowToggleFloating CommandID = "window.toggle_floating"
	CommandWindowZoom           CommandID = "window.zoom"
	CommandLayoutArrange        CommandID = "layout.arrange"
	CommandLayoutResize         CommandID = "layout.resize"
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
	// ClientID is required for desktop.switch, window.focus, and window.zoom.
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
	DesktopID  DesktopID
	Name       string
	Purpose    DesktopPurpose
	FocusOwner *WindowID
	AfterID    *DesktopID
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
	ID           WindowID
	App          string
	InstanceKey  *string
	Route        RouteIntent
	DesktopID    DesktopID
	FloatingRect NormalizedRect
	InsertTiled  bool
}

type OpenWindowCommand struct {
	Window          WindowSpec
	RestoreWindowID *WindowID
}

func (OpenWindowCommand) CommandID() CommandID { return CommandWindowOpen }

type NavigateWindowCommand struct {
	WindowID WindowID
	Route    RouteIntent
}

func (NavigateWindowCommand) CommandID() CommandID { return CommandWindowNavigate }

// CloseWindowCommand minimizes instead of deleting when Minimize is true.
type CloseWindowCommand struct {
	WindowID WindowID
	Minimize bool
}

func (CloseWindowCommand) CommandID() CommandID { return CommandWindowClose }

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
	DesktopIDs []DesktopID `json:"desktop_ids,omitempty"`
	WindowIDs  []WindowID  `json:"window_ids,omitempty"`
	GroupIDs   []GroupID   `json:"group_ids,omitempty"`
	NodeIDs    []NodeID    `json:"node_ids,omitempty"`
	ClientIDs  []ClientID  `json:"client_ids,omitempty"`
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
