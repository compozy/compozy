package hooks

// WindowManagerChanges identifies entities changed by one committed command.
type WindowManagerChanges struct {
	TabIDs         []string `json:"tab_ids,omitempty"`
	TabSplit       []string `json:"tab_split,omitempty"`
	TabUnsplit     []string `json:"tab_unsplit,omitempty"`
	DesktopIDs     []string `json:"desktop_ids,omitempty"`
	WindowIDs      []string `json:"window_ids,omitempty"`
	GroupIDs       []string `json:"group_ids,omitempty"`
	NodeIDs        []string `json:"node_ids,omitempty"`
	ClientIDs      []string `json:"client_ids,omitempty"`
	StackGrouped   []string `json:"stack_grouped,omitempty"`
	StackUngrouped []string `json:"stack_ungrouped,omitempty"`
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

// WindowManagerWindowOpenedPayload observes windows added by open or reopen.
type WindowManagerWindowOpenedPayload = WindowManagerPayload

// WindowManagerWindowClosedPayload observes every window removed by one close operation.
type WindowManagerWindowClosedPayload = WindowManagerPayload

// WindowManagerStackGroupedPayload observes stack membership grouped in one revision.
type WindowManagerStackGroupedPayload = WindowManagerPayload

// WindowManagerStackUngroupedPayload observes stacks dissolved in one revision.
type WindowManagerStackUngroupedPayload = WindowManagerPayload

// WindowManagerStackActivatedPayload observes one durable stack activation.
type WindowManagerStackActivatedPayload = WindowManagerPayload

// WindowManagerObservationPatch is the no-op patch surface for durable observations.
type WindowManagerObservationPatch struct{}

// WindowManagerTabSplitPayload observes tabs that gained split members.
type WindowManagerTabSplitPayload = WindowManagerPayload

// WindowManagerTabUnsplitPayload observes split tabs reduced to one member or dissolved.
type WindowManagerTabUnsplitPayload = WindowManagerPayload

// WindowManagerTabActivatedPayload observes explicit and coalesced tab activation.
type WindowManagerTabActivatedPayload = WindowManagerPayload
