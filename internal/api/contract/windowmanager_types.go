package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/windowmanager"
)

// WindowManagerRevision is an unsigned wire integer that remains exact in JavaScript.
type WindowManagerRevision uint64

// WindowManagerMaxSafeRevision is the largest integer exactly representable by JavaScript numbers.
const WindowManagerMaxSafeRevision WindowManagerRevision = WindowManagerRevision(windowmanager.MaxWireRevision)

// ErrWindowManagerUnsafeRevision identifies a domain revision that cannot be represented on the wire.
var ErrWindowManagerUnsafeRevision = errors.New("window manager revision exceeds the JSON safe integer range")

// UnmarshalJSON rejects revision values that JavaScript clients cannot represent exactly.
func (revision *WindowManagerRevision) UnmarshalJSON(data []byte) error {
	var raw uint64
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode window manager revision: %w", err)
	}
	if raw > uint64(WindowManagerMaxSafeRevision) {
		return ErrWindowManagerUnsafeRevision
	}
	*revision = WindowManagerRevision(raw)
	return nil
}

// WindowManagerErrorCode is a stable failure identifier shared by every transport.
type WindowManagerErrorCode string

const (
	WindowManagerErrorWorkspaceNotFound WindowManagerErrorCode = "window_manager_workspace_not_found"
	WindowManagerErrorRevisionConflict  WindowManagerErrorCode = "window_manager_revision_conflict"
	WindowManagerErrorBindingInvalid    WindowManagerErrorCode = "window_binding_invalid"
	WindowManagerErrorInvalidCommand    WindowManagerErrorCode = "window_manager_invalid_command"
	WindowManagerErrorInvalidTopology   WindowManagerErrorCode = "window_manager_invalid_topology"
	WindowManagerErrorDesktopNotFound   WindowManagerErrorCode = "window_manager_desktop_not_found"
	WindowManagerErrorWindowNotFound    WindowManagerErrorCode = "window_manager_window_not_found"
	WindowManagerErrorNotStacked        WindowManagerErrorCode = "window_manager_not_stacked"
	WindowManagerErrorWindowPinned      WindowManagerErrorCode = "window_manager_window_pinned"
	WindowManagerErrorClientNotFound    WindowManagerErrorCode = "window_manager_client_not_found"
	WindowManagerErrorDestination       WindowManagerErrorCode = "window_manager_destination_required"
	WindowManagerErrorFinalDesktop      WindowManagerErrorCode = "window_manager_final_desktop"
	WindowManagerErrorHistoryBoundary   WindowManagerErrorCode = "window_manager_history_boundary"
	WindowManagerErrorLayoutNotFound    WindowManagerErrorCode = "window_manager_layout_not_found"
	WindowManagerErrorSlowConsumer      WindowManagerErrorCode = "window_manager_slow_consumer"
	WindowManagerErrorUnavailable       WindowManagerErrorCode = "window_manager_unavailable"
)

// WindowManagerErrorCodeValues returns every public window-manager error code.
func WindowManagerErrorCodeValues() []string {
	return []string{
		string(WindowManagerErrorWorkspaceNotFound),
		string(WindowManagerErrorRevisionConflict),
		string(WindowManagerErrorBindingInvalid),
		string(WindowManagerErrorInvalidCommand),
		string(WindowManagerErrorInvalidTopology),
		string(WindowManagerErrorDesktopNotFound),
		string(WindowManagerErrorWindowNotFound),
		string(WindowManagerErrorNotStacked),
		string(WindowManagerErrorWindowPinned),
		string(WindowManagerErrorClientNotFound),
		string(WindowManagerErrorDestination),
		string(WindowManagerErrorFinalDesktop),
		string(WindowManagerErrorHistoryBoundary),
		string(WindowManagerErrorLayoutNotFound),
		string(WindowManagerErrorSlowConsumer),
		string(WindowManagerErrorUnavailable),
	}
}

// WindowManagerErrorPayload is the deterministic error body shared by HTTP, UDS, CLI, and WebSocket.
type WindowManagerErrorPayload struct {
	Error           string                     `json:"error"`
	Code            WindowManagerErrorCode     `json:"code"`
	WorkspaceID     windowmanager.WorkspaceID  `json:"workspace_id"`
	CurrentRevision *WindowManagerRevision     `json:"current_revision,omitzero"`
	Conflicts       []windowmanager.Conflict   `json:"conflicts,omitempty"`
	Diagnostics     []windowmanager.Diagnostic `json:"diagnostics,omitempty"`
}

// WindowManagerActor records a command initiator without affecting command semantics.
type WindowManagerActor struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

type WindowManagerWindow struct {
	ID          windowmanager.WindowID      `json:"id"`
	App         string                      `json:"app"`
	InstanceKey *string                     `json:"instance_key,omitzero"`
	Route       windowmanager.RouteIntent   `json:"route"`
	NavStack    []windowmanager.RouteIntent `json:"nav_stack"`
	DesktopID   windowmanager.DesktopID     `json:"desktop_id"`
	TabID       windowmanager.TabID         `json:"tab_id"`
}

// WindowManagerSnapshot is the complete durable workspace aggregate.
type WindowManagerSnapshot struct {
	Version          uint32                         `json:"version"`
	WorkspaceID      windowmanager.WorkspaceID      `json:"workspace_id"`
	Revision         WindowManagerRevision          `json:"revision"`
	Desktops         []WindowManagerDesktop         `json:"desktops"`
	Windows          map[string]WindowManagerWindow `json:"windows"`
	Migration        *windowmanager.MigrationStatus `json:"migration,omitzero"`
	ClosedEntryCount int                            `json:"closed_entry_count"`
	Overrides        windowmanager.WorkspaceConfig  `json:"overrides"`
	UpdatedAt        time.Time                      `json:"updated_at"`
}

// WindowManagerLayoutDocument is the stable declarative layout wire contract.
type WindowManagerLayoutDocument struct {
	inputOverrides    json.RawMessage                `json:"-"`
	InputDeprecations []windowmanager.Deprecation    `json:"-"`
	Version           uint32                         `json:"version"`
	WorkspaceID       windowmanager.WorkspaceID      `json:"workspace_id"`
	Desktops          []WindowManagerDesktop         `json:"desktops"`
	Windows           map[string]WindowManagerWindow `json:"windows"`
	Overrides         windowmanager.WorkspaceConfig  `json:"overrides"`
}

// WindowManagerClientView is transient presentation state for one connected client.
type WindowManagerClientView struct {
	WorkspaceID          windowmanager.WorkspaceID                  `json:"workspace_id"`
	ClientID             windowmanager.ClientID                     `json:"client_id"`
	Kind                 windowmanager.ClientKind                   `json:"kind"`
	PresentationRevision WindowManagerRevision                      `json:"presentation_revision"`
	ContextRevision      WindowManagerRevision                      `json:"context_revision"`
	ActiveDesktopID      windowmanager.DesktopID                    `json:"active_desktop_id"`
	FocusedWindowID      *windowmanager.WindowID                    `json:"focused_window_id,omitzero"`
	FocusOrder           []windowmanager.WindowID                   `json:"focus_order"`
	ActiveTab            map[string]string                          `json:"active_tab"`
	StackActive          map[string]string                          `json:"stack_active"               deprecated:"true"`
	PaletteContext       WindowManagerPaletteContext                `json:"palette_context"`
	GlobalShortcuts      []windowmanager.GlobalShortcutRegistration `json:"global_shortcuts"`
	ConnectedAt          time.Time                                  `json:"connected_at"`
	AttachmentToken      string                                     `json:"attachment_token,omitempty"`
}

type WindowManagerPaletteContext struct {
	WindowFocused       bool                       `json:"window_focused"`
	WindowFloating      bool                       `json:"window_floating"                 deprecated:"true"`
	WindowInSplit       bool                       `json:"window_in_split"`
	WindowStacked       bool                       `json:"window_stacked"                  deprecated:"true"`
	DesktopWindowCount  int                        `json:"desktop_window_count"`
	ScopeGlobal         bool                       `json:"scope_global"`
	ShellDesktop        bool                       `json:"shell_desktop"`
	FocusedSessionState string                     `json:"focused_session_state,omitempty"`
	WorkspaceTrusted    bool                       `json:"workspace_trusted"`
	DestinationIntent   *windowmanager.RouteIntent `json:"destination_intent,omitzero"`
}

// WindowManagerClientsResponse lists client-local views in one workspace partition.
type WindowManagerClientsResponse struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Clients     []WindowManagerClientView `json:"clients"`
}

// WindowManagerClientRegistration requests one live client view.
type WindowManagerClientRegistration struct {
	WorkspaceID     windowmanager.WorkspaceID       `json:"workspace_id"`
	ClientID        windowmanager.ClientID          `json:"client_id,omitempty"`
	Kind            windowmanager.ClientKind        `json:"kind,omitempty"`
	ActiveDesktopID windowmanager.DesktopID         `json:"active_desktop_id,omitempty"`
	Context         WindowManagerClientContextInput `json:"context"`
}

type WindowManagerClientContextInput struct {
	ScopeGlobal         bool                                       `json:"scope_global"`
	FocusedSessionState string                                     `json:"focused_session_state,omitempty"`
	WorkspaceTrusted    bool                                       `json:"workspace_trusted"`
	DestinationIntent   *windowmanager.RouteIntent                 `json:"destination_intent,omitzero"`
	GlobalShortcuts     []windowmanager.GlobalShortcutRegistration `json:"global_shortcuts"`
}

// WindowManagerLayoutValidationRequest validates a declarative document without writing it.
type WindowManagerLayoutValidationRequest struct {
	WorkspaceID windowmanager.WorkspaceID   `json:"workspace_id"`
	Document    WindowManagerLayoutDocument `json:"document"`
}

// WindowManagerLayoutValidationResponse returns diagnostics and the canonical document when valid.
type WindowManagerLayoutValidationResponse struct {
	Document    *WindowManagerLayoutDocument `json:"document,omitzero"`
	WorkspaceID windowmanager.WorkspaceID    `json:"workspace_id"`
	Valid       bool                         `json:"valid"`
	Diagnostics []windowmanager.Diagnostic   `json:"diagnostics,omitempty"`
}

// WindowManagerLayoutReplaceRequest applies a declarative document through revision CAS.
type WindowManagerLayoutReplaceRequest struct {
	WorkspaceID      windowmanager.WorkspaceID   `json:"workspace_id"`
	ExpectedRevision *WindowManagerRevision      `json:"expected_revision"`
	ClientID         *windowmanager.ClientID     `json:"client_id,omitzero"`
	Actor            WindowManagerActor          `json:"actor"`
	Origin           string                      `json:"origin"`
	Document         WindowManagerLayoutDocument `json:"document"`
}

// WindowManagerResult reports one applied or no-op command.
type WindowManagerResult struct {
	Deprecations []windowmanager.Deprecation `json:"deprecations"`
	Snapshot     WindowManagerSnapshot       `json:"snapshot"`
	Applied      bool                        `json:"applied"`
	Changes      windowmanager.ChangeSet     `json:"changes"`
	Diagnostics  []windowmanager.Diagnostic  `json:"diagnostics,omitempty"`
	Client       *WindowManagerClientView    `json:"client,omitzero"`
	RebasedFrom  *WindowManagerRevision      `json:"rebased_from,omitzero"`
}

// WindowManagerPreview reports a validated proposal without a durable write.
type WindowManagerPreview struct {
	Deprecations []windowmanager.Deprecation `json:"deprecations"`
	Snapshot     WindowManagerSnapshot       `json:"snapshot"`
	Changed      bool                        `json:"changed"`
	Changes      windowmanager.ChangeSet     `json:"changes"`
	Diagnostics  []windowmanager.Diagnostic  `json:"diagnostics,omitempty"`
	Client       *WindowManagerClientView    `json:"client,omitzero"`
}
