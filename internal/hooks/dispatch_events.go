package hooks

import (
	"context"
	"strings"
	"time"
)

const (
	dispatchEventsAgentSessionKey = "agent_session"
	dispatchEventsNetworkPeerKey  = "network_peer"
)

// DispatchPhase identifies whether a hook-dispatch event marks pipeline entry
// or one completed hook execution.
type DispatchPhase string

const (
	DispatchPhaseStart    DispatchPhase = "start"
	DispatchPhaseComplete DispatchPhase = "complete"
)

// DispatchEventEmitter emits canonical hook.dispatch.* events for public
// session and observe surfaces.
type DispatchEventEmitter interface {
	EmitHookDispatchEvent(
		ctx context.Context,
		payload any,
		hook RegisteredHook,
		phase DispatchPhase,
		outcome HookRunOutcome,
		err error,
		depth int,
		timestamp time.Time,
	)
}

type dispatchEventEmitterContextKey struct{}

// WithDispatchEventEmitter attaches a canonical dispatch-event emitter to the context.
func WithDispatchEventEmitter(ctx context.Context, emitter DispatchEventEmitter) context.Context {
	if ctx == nil || emitter == nil {
		return ctx
	}
	return context.WithValue(ctx, dispatchEventEmitterContextKey{}, emitter)
}

// DispatchEventEmitterFromContext resolves the attached dispatch-event emitter.
func DispatchEventEmitterFromContext(ctx context.Context) DispatchEventEmitter {
	if ctx == nil {
		return nil
	}
	emitter, ok := ctx.Value(dispatchEventEmitterContextKey{}).(DispatchEventEmitter)
	if !ok {
		return nil
	}
	return emitter
}

// DispatchCorrelation carries the public correlation keys extractable from one
// typed hook payload without importing the store package.
type DispatchCorrelation struct {
	TaskID               string
	RunID                string
	LoopRunID            string
	LoopName             string
	NodeID               string
	WorkflowID           string
	CoordinatorSessionID string
	ActorKind            string
	ActorID              string
	ReleaseReason        string
}

// SessionContextFromPayload extracts the shared session context from a typed hook payload.
func SessionContextFromPayload(payload any) SessionContext {
	carrier, ok := payload.(sessionContextCarrier)
	if !ok {
		return SessionContext{}
	}
	return carrier.hookSessionContext()
}

// TurnIDFromPayload extracts the current turn identifier from typed payloads when present.
func TurnIDFromPayload(payload any) string {
	switch typed := payload.(type) {
	case InputPreSubmitPayload:
		return strings.TrimSpace(typed.TurnID)
	case PromptPayload:
		return strings.TrimSpace(typed.TurnID)
	case EventRecordPayload:
		return strings.TrimSpace(typed.TurnID)
	case SessionMessagePersistedPayload:
		return strings.TrimSpace(typed.TurnID)
	case TurnPayload:
		return strings.TrimSpace(typed.TurnID)
	case MessagePayload:
		return strings.TrimSpace(typed.TurnID)
	case ToolPreCallPayload:
		return strings.TrimSpace(typed.TurnID)
	case ToolPostCallPayload:
		return strings.TrimSpace(typed.TurnID)
	case ToolPostErrorPayload:
		return strings.TrimSpace(typed.TurnID)
	case PermissionRequestPayload:
		return strings.TrimSpace(typed.TurnID)
	case PermissionResolutionPayload:
		return strings.TrimSpace(typed.TurnID)
	case ContextCompactPayload:
		return strings.TrimSpace(typed.TurnID)
	default:
		return ""
	}
}

// CorrelationFromPayload extracts the correlation fields that are owned by the
// typed hook payload itself.
func CorrelationFromPayload(payload any) DispatchCorrelation {
	switch typed := payload.(type) {
	case CoordinatorPreSpawnPayload:
		return DispatchCorrelation{
			CoordinatorSessionID: strings.TrimSpace(typed.CoordinatorSessionID),
			WorkflowID:           strings.TrimSpace(typed.WorkflowID),
			ActorKind:            dispatchEventsAgentSessionKey,
			ActorID:              strings.TrimSpace(typed.CoordinatorSessionID),
		}
	case CoordinatorLifecyclePayload:
		return DispatchCorrelation{
			CoordinatorSessionID: strings.TrimSpace(typed.CoordinatorSessionID),
			WorkflowID:           strings.TrimSpace(typed.WorkflowID),
			ActorKind:            dispatchEventsAgentSessionKey,
			ActorID:              strings.TrimSpace(typed.CoordinatorSessionID),
		}
	case TaskBlockPayload:
		return correlationFromTaskContext(typed.TaskContext)
	case TaskAttentionPayload:
		return correlationFromTaskContext(typed.TaskContext)
	case TaskStatusChangedPayload:
		return correlationFromTaskContext(typed.TaskContext)
	case TaskRunEnqueuedPayload:
		return correlationFromTaskRunContext(typed.TaskRunContext)
	case TaskRunPreClaimPayload:
		return correlationFromTaskRunContext(typed.taskRunContextSnapshot())
	case TaskRunPostClaimPayload:
		return correlationFromTaskRunContext(typed.TaskRunContext)
	case TaskRunLeasePayload:
		return correlationFromTaskRunContext(typed.TaskRunContext)
	case LoopLifecyclePayload:
		return correlationFromLoopContext(typed.LoopContext)
	case LoopGenerationPayload:
		return correlationFromLoopContext(typed.LoopContext)
	case LoopGatePayload:
		return correlationFromLoopContext(typed.LoopContext)
	case LoopNodeTerminalPayload:
		return correlationFromLoopContext(typed.LoopContext)
	case SpawnPreCreatePayload:
		return correlationFromSpawnContext(typed.SpawnContext)
	case SpawnLifecyclePayload:
		return correlationFromSpawnContext(typed.SpawnContext)
	case NetworkPayload:
		actorID := strings.TrimSpace(typed.PeerFrom)
		if actorID == "" {
			actorID = strings.TrimSpace(typed.SessionID)
		}
		if actorID == "" {
			return DispatchCorrelation{}
		}
		return DispatchCorrelation{
			ActorKind: dispatchEventsNetworkPeerKey,
			ActorID:   actorID,
		}
	default:
		sessionCtx := SessionContextFromPayload(payload)
		sessionID := strings.TrimSpace(sessionCtx.SessionID)
		if sessionID == "" {
			return DispatchCorrelation{}
		}
		return DispatchCorrelation{
			ActorKind: dispatchEventsAgentSessionKey,
			ActorID:   sessionID,
		}
	}
}

func correlationFromTaskContext(ctx TaskContext) DispatchCorrelation {
	return DispatchCorrelation{
		TaskID:        strings.TrimSpace(ctx.TaskID),
		RunID:         strings.TrimSpace(ctx.RunID),
		WorkflowID:    strings.TrimSpace(ctx.WorkflowID),
		ActorKind:     strings.TrimSpace(ctx.ActorKind),
		ActorID:       strings.TrimSpace(ctx.ActorID),
		ReleaseReason: strings.TrimSpace(ctx.ReleaseReason),
	}
}

func correlationFromTaskRunContext(ctx TaskRunContext) DispatchCorrelation {
	return DispatchCorrelation{
		TaskID:               strings.TrimSpace(ctx.TaskID),
		RunID:                strings.TrimSpace(ctx.RunID),
		LoopRunID:            strings.TrimSpace(ctx.LoopRunID),
		WorkflowID:           strings.TrimSpace(ctx.WorkflowID),
		ActorKind:            strings.TrimSpace(ctx.ActorKind),
		ActorID:              strings.TrimSpace(ctx.ActorID),
		ReleaseReason:        strings.TrimSpace(ctx.ReleaseReason),
		CoordinatorSessionID: strings.TrimSpace(ctx.SessionID),
	}
}

func correlationFromLoopContext(ctx LoopContext) DispatchCorrelation {
	return DispatchCorrelation{
		TaskID:               strings.TrimSpace(ctx.TaskID),
		RunID:                strings.TrimSpace(ctx.RunID),
		LoopRunID:            strings.TrimSpace(ctx.LoopRunID),
		LoopName:             strings.TrimSpace(ctx.LoopName),
		NodeID:               strings.TrimSpace(ctx.NodeID),
		WorkflowID:           strings.TrimSpace(ctx.WorkflowID),
		CoordinatorSessionID: strings.TrimSpace(ctx.SessionID),
		ActorKind:            strings.TrimSpace(ctx.ActorKind),
		ActorID:              strings.TrimSpace(ctx.ActorID),
	}
}

func correlationFromSpawnContext(ctx SpawnContext) DispatchCorrelation {
	actorID := strings.TrimSpace(ctx.ChildSessionID)
	if actorID == "" {
		actorID = strings.TrimSpace(ctx.ParentSessionID)
	}
	return DispatchCorrelation{
		TaskID:     strings.TrimSpace(ctx.TaskID),
		RunID:      strings.TrimSpace(ctx.RunID),
		WorkflowID: strings.TrimSpace(ctx.WorkflowID),
		ActorKind:  dispatchEventsAgentSessionKey,
		ActorID:    actorID,
	}
}
