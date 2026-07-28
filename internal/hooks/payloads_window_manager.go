package hooks

// WindowManagerChanges identifies entities changed by one committed command.
type WindowManagerChanges struct {
	DesktopIDs []string `json:"desktop_ids,omitempty"`
	WindowIDs  []string `json:"window_ids,omitempty"`
	GroupIDs   []string `json:"group_ids,omitempty"`
	NodeIDs    []string `json:"node_ids,omitempty"`
	ClientIDs  []string `json:"client_ids,omitempty"`
}

// WindowManagerActor identifies the command initiator.
type WindowManagerActor struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

// WindowManagerPayload observes one durable topology mutation.
type WindowManagerPayload struct {
	PayloadBase
	WorkspaceID string               `json:"workspace_id"`
	Revision    uint64               `json:"revision"`
	CommandID   string               `json:"command_id"`
	Changes     WindowManagerChanges `json:"changes"`
	Actor       WindowManagerActor   `json:"actor"`
	Origin      string               `json:"origin,omitempty"`
}

// WindowManagerLayoutAppliedPayload observes a committed layout application.
type WindowManagerLayoutAppliedPayload = WindowManagerPayload

// WindowManagerDesktopCreatedPayload observes a committed desktop creation.
type WindowManagerDesktopCreatedPayload = WindowManagerPayload

// WindowManagerDesktopDeletedPayload observes a committed desktop deletion.
type WindowManagerDesktopDeletedPayload = WindowManagerPayload

// WindowManagerWindowMovedPayload observes a committed window move.
type WindowManagerWindowMovedPayload = WindowManagerPayload

// WindowManagerObservationPatch is the no-op patch surface for durable observations.
type WindowManagerObservationPatch struct{}
