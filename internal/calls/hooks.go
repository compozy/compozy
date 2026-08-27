package calls

import "context"

// HookEvent identifies a committed call transition observable by the daemon.
type HookEvent string

const (
	// HookCallCreated and the following constants identify observable call transitions.
	HookCallCreated          HookEvent = "call.created"
	HookCallStateChanged     HookEvent = "call.state_changed"
	HookCallSettled          HookEvent = "call.settled"
	HookCallCanceled         HookEvent = "call.canceled"
	HookCallPublished        HookEvent = "call.published"
	HookCallMessageSent      HookEvent = "call.message_sent"
	HookCallMessageDelivered HookEvent = "call.message_delivered"
	HookCallMessageRejected  HookEvent = "call.message_rejected"
	HookCallRevived          HookEvent = "call.revived"
	HookCallReaped           HookEvent = "call.reaped"
	HookCallSubtreeDrained   HookEvent = "call.subtree_drained"
)

// HookPayload contains identifiers and lifecycle facts only. Prompts, message bodies,
// results, and diagnostics never cross the observation boundary.
type HookPayload struct {
	ProfileID        string
	Scope            Scope
	WorkspaceID      string
	CallID           string
	MessageID        string
	ParentSessionID  string
	ChildSessionID   string
	RootSessionID    string
	AgentName        string
	PreviousState    State
	State            State
	Reason           string
	Verdict          Verdict
	Actor            Actor
	Channel          string
	ThreadID         string
	NetworkMessageID string
	Delivery         string
	StoppedChildren  int
	ClosedCalls      int
	PreservedResults int
}

// HookDispatcher observes committed transitions. Implementations must be fail-open.
type HookDispatcher interface {
	ObserveCall(context.Context, HookEvent, HookPayload)
}

func (s *Service) emitHook(ctx context.Context, event HookEvent, payload HookPayload) {
	if s.hooks == nil {
		return
	}
	s.hooks.ObserveCall(ctx, event, payload)
}

func hookPayloadForCall(record *CallRecord) HookPayload {
	return HookPayload{
		ProfileID: record.ProfileID, Scope: record.Scope, WorkspaceID: record.WorkspaceID,
		CallID: record.CallID, ParentSessionID: record.ParentSessionID,
		ChildSessionID: record.ChildSessionID, RootSessionID: record.GovernedRootID,
		AgentName: record.AgentName, State: record.State, Verdict: record.Verdict,
		Actor: record.Actor,
	}
}

func (s *Service) emitStateChanged(ctx context.Context, previous State, record *CallRecord) {
	payload := hookPayloadForCall(record)
	payload.PreviousState = previous
	s.emitHook(ctx, HookCallStateChanged, payload)
}

func (s *Service) emitTerminalTransition(ctx context.Context, previous State, record *CallRecord) {
	s.emitStateChanged(ctx, previous, record)
	s.emitHook(ctx, HookCallSettled, hookPayloadForCall(record))
}
