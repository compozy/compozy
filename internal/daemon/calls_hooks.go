package daemon

import (
	"context"
	"log/slog"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

type daemonCallHookDispatcher struct {
	state            *bootState
	logger           *slog.Logger
	now              func() time.Time
	operationTimeout time.Duration
}

func (d daemonCallHookDispatcher) ObserveCall(
	ctx context.Context,
	event callspkg.HookEvent,
	payload callspkg.HookPayload,
) {
	if d.state == nil || d.state.hookDispatcher == nil {
		return
	}
	hooks := d.state.hookDispatcher
	dispatchCtx, cancel := detachedDaemonOperationContext(ctx, d.operationTimeout)
	defer cancel()
	hookEvent := hookspkg.HookEvent(event)
	now := time.Now().UTC()
	if d.now != nil {
		now = d.now().UTC()
	}
	hookPayload := hookspkg.CallObservationPayload{
		PayloadBase: hookspkg.PayloadBase{Event: hookEvent, Timestamp: now},
		ProfileID:   payload.ProfileID, Scope: string(payload.Scope), WorkspaceID: payload.WorkspaceID,
		CallID: payload.CallID, MessageID: payload.MessageID,
		ParentSessionID: payload.ParentSessionID, ChildSessionID: payload.ChildSessionID,
		RootSessionID: payload.RootSessionID, AgentName: payload.AgentName,
		PreviousState: string(payload.PreviousState), State: string(payload.State),
		Reason: payload.Reason, Verdict: string(payload.Verdict),
		ActorKind: payload.Actor.Kind, ActorID: payload.Actor.ID,
		Channel: payload.Channel, ThreadID: payload.ThreadID,
		NetworkMessageID: payload.NetworkMessageID, Delivery: payload.Delivery,
		StoppedChildren: payload.StoppedChildren, ClosedCalls: payload.ClosedCalls,
		PreservedResults: payload.PreservedResults,
	}
	if err := dispatchCallHook(dispatchCtx, hooks, hookEvent, hookPayload); err != nil {
		logger := d.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.WarnContext(
			dispatchCtx,
			"call.hook_dispatch_failed",
			"hook_event", hookEvent,
			"call_id", payload.CallID,
			"message_id", payload.MessageID,
			"error", err,
		)
	}
}

func dispatchCallHook(
	ctx context.Context,
	dispatcher *hookspkg.Hooks,
	event hookspkg.HookEvent,
	payload hookspkg.CallObservationPayload,
) error {
	var err error
	switch event {
	case hookspkg.HookCallCreated:
		_, err = dispatcher.DispatchCallCreated(ctx, payload)
	case hookspkg.HookCallStateChanged:
		_, err = dispatcher.DispatchCallStateChanged(ctx, payload)
	case hookspkg.HookCallSettled:
		_, err = dispatcher.DispatchCallSettled(ctx, payload)
	case hookspkg.HookCallCanceled:
		_, err = dispatcher.DispatchCallCanceled(ctx, payload)
	case hookspkg.HookCallPublished:
		_, err = dispatcher.DispatchCallPublished(ctx, payload)
	case hookspkg.HookCallMessageSent:
		_, err = dispatcher.DispatchCallMessageSent(ctx, payload)
	case hookspkg.HookCallMessageDelivered:
		_, err = dispatcher.DispatchCallMessageDelivered(ctx, payload)
	case hookspkg.HookCallMessageRejected:
		_, err = dispatcher.DispatchCallMessageRejected(ctx, payload)
	case hookspkg.HookCallRevived:
		_, err = dispatcher.DispatchCallRevived(ctx, payload)
	case hookspkg.HookCallReaped:
		_, err = dispatcher.DispatchCallReaped(ctx, payload)
	case hookspkg.HookCallSubtreeDrained:
		_, err = dispatcher.DispatchCallSubtreeDrained(ctx, payload)
	}
	return err
}
