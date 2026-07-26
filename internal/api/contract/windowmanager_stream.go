package contract

import (
	"time"

	"github.com/compozy/agh/internal/windowmanager"
)

const (
	WindowManagerFrameSnapshot = "snapshot"
	WindowManagerFrameEvent    = "event"
	WindowManagerFrameClient   = "client"
	WindowManagerFrameError    = "error"
)

// WindowManagerEvent is one committed semantic mutation.
type WindowManagerEvent struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    WindowManagerRevision     `json:"revision"`
	CommandID   WindowManagerCommandID    `json:"command_id"`
	Changes     windowmanager.ChangeSet   `json:"changes"`
	Actor       WindowManagerActor        `json:"actor"`
	Origin      string                    `json:"origin,omitempty"`
	OccurredAt  time.Time                 `json:"occurred_at"`
}

// WindowManagerSnapshotFrame is the first frame of every stream subscription.
type WindowManagerSnapshotFrame struct {
	Type        string                    `json:"type"`
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    WindowManagerRevision     `json:"revision"`
	Snapshot    WindowManagerSnapshot     `json:"snapshot"`
	Client      *WindowManagerClientView  `json:"client,omitempty"`
}

// WindowManagerEventFrame carries one event strictly after the snapshot fence.
type WindowManagerEventFrame struct {
	Type        string                    `json:"type"`
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    WindowManagerRevision     `json:"revision"`
	Event       WindowManagerEvent        `json:"event"`
}

// WindowManagerClientFrame carries one bound client's presentation change.
type WindowManagerClientFrame struct {
	Type        string                    `json:"type"`
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    WindowManagerRevision     `json:"revision"`
	Client      WindowManagerClientView   `json:"client"`
}

// WindowManagerErrorFrame closes a stream with one stable structured error.
type WindowManagerErrorFrame struct {
	Type  string                    `json:"type"`
	Error WindowManagerErrorPayload `json:"error"`
}

// WindowManagerEventFromDomain converts a committed domain event to the stable wire form.
func WindowManagerEventFromDomain(event windowmanager.Event) (WindowManagerEvent, error) {
	revision, err := windowManagerRevision(event.Revision)
	if err != nil {
		return WindowManagerEvent{}, err
	}
	return WindowManagerEvent{
		WorkspaceID: event.WorkspaceID, Revision: revision,
		CommandID: WindowManagerCommandID(event.CommandID), Changes: event.Changes,
		Actor:  WindowManagerActor{Kind: event.Actor.Kind, ID: event.Actor.ID},
		Origin: event.Origin, OccurredAt: event.OccurredAt,
	}, nil
}
