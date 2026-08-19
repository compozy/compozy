package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	WindowManagerFrameSnapshot            = "snapshot"
	WindowManagerFrameEvent               = "event"
	WindowManagerFrameClient              = "client"
	WindowManagerFrameClientCommand       = "client_command"
	WindowManagerFrameClientCommandAck    = "client_command_ack"
	WindowManagerFrameClientCommandResult = "client_command_result"
	WindowManagerFrameClientContext       = "client_context"
	WindowManagerFrameError               = "error"
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

// WindowManagerClientCommandFrame carries one daemon-to-client UI operation.
type WindowManagerClientCommandFrame struct {
	Type        string                    `json:"type"`
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	CommandID   string                    `json:"command_id"`
	Op          string                    `json:"op"`
	Payload     json.RawMessage           `json:"payload,omitempty"`
}

// WindowManagerClientCommandAckFrame acknowledges delivery before client execution completes.
type WindowManagerClientCommandAckFrame struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
}

// WindowManagerClientCommandResultFrame terminates one correlated client operation.
type WindowManagerClientCommandResultFrame struct {
	Type      string          `json:"type"`
	CommandID string          `json:"command_id"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// WindowManagerClientContextFrame replaces the client-owned palette context on one bound stream.
type WindowManagerClientContextFrame struct {
	Type    string                          `json:"type"`
	Context WindowManagerClientContextInput `json:"context"`
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
