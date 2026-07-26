package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

type hookEventSummaryWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type globalHookDispatchEventEmitter struct {
	summaries hookEventSummaryWriter
}

var _ hookspkg.DispatchEventEmitter = globalHookDispatchEventEmitter{}

func withGlobalHookDispatchEventEmitter[P any](
	ctx context.Context,
	notifier *hooksNotifier,
	payload P,
) context.Context {
	if ctx == nil || notifier == nil {
		return ctx
	}
	if hookspkg.DispatchEventEmitterFromContext(ctx) != nil {
		return ctx
	}
	if !supportsGlobalHookDispatchEvents(payload) {
		return ctx
	}

	summaries := notifier.hookEventSummaries()
	if summaries == nil {
		return ctx
	}
	return hookspkg.WithDispatchEventEmitter(ctx, globalHookDispatchEventEmitter{
		summaries: summaries,
	})
}

func supportsGlobalHookDispatchEvents(payload any) bool {
	switch payload.(type) {
	case hookspkg.CoordinatorPreSpawnPayload:
		return true
	case hookspkg.CoordinatorLifecyclePayload:
		return true
	case hookspkg.TaskRunEnqueuedPayload:
		return true
	case hookspkg.TaskRunPreClaimPayload:
		return true
	case hookspkg.TaskRunPostClaimPayload:
		return true
	case hookspkg.TaskRunLeasePayload:
		return true
	default:
		return false
	}
}

func (e globalHookDispatchEventEmitter) EmitHookDispatchEvent(
	ctx context.Context,
	payload any,
	hook hookspkg.RegisteredHook,
	phase hookspkg.DispatchPhase,
	outcome hookspkg.HookRunOutcome,
	err error,
	_ int,
	timestamp time.Time,
) {
	if e.summaries == nil {
		return
	}

	sessionCtx := hookspkg.SessionContextFromPayload(payload)
	correlation := hookspkg.CorrelationFromPayload(payload)
	eventCorrelation := store.EventCorrelation{
		TaskID:               correlation.TaskID,
		RunID:                correlation.RunID,
		WorkflowID:           correlation.WorkflowID,
		CoordinatorSessionID: correlation.CoordinatorSessionID,
		HookEvent:            hook.Event.String(),
		HookName:             strings.TrimSpace(hook.Name),
		ActorKind:            correlation.ActorKind,
		ActorID:              correlation.ActorID,
		ReleaseReason:        correlation.ReleaseReason,
	}.Normalize()

	content, marshalErr := json.Marshal(globalHookDispatchContent{
		HookEvent: hook.Event.String(),
		HookName:  strings.TrimSpace(hook.Name),
		Phase:     string(phase),
		Outcome:   strings.TrimSpace(string(outcome)),
		Decision:  strings.TrimSpace(string(outcome)),
		Error:     hookDispatchErrorString(err),
	})
	if marshalErr != nil {
		return
	}

	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	if writeErr := e.summaries.WriteEventSummary(ctx, store.EventSummary{
		SessionID:        strings.TrimSpace(sessionCtx.SessionID),
		Type:             hookDispatchEventType(phase),
		AgentName:        strings.TrimSpace(sessionCtx.AgentName),
		Content:          content,
		EventCorrelation: eventCorrelation,
		Summary:          globalHookDispatchSummary(hook, phase, outcome),
		Timestamp:        timestamp.UTC(),
	}); writeErr != nil {
		return
	}
}

func hookDispatchEventType(phase hookspkg.DispatchPhase) string {
	switch phase {
	case hookspkg.DispatchPhaseStart:
		return eventspkg.HookDispatchStart
	case hookspkg.DispatchPhaseComplete:
		return eventspkg.HookDispatchComplete
	default:
		return "hook.dispatch." + string(phase)
	}
}

type globalHookDispatchContent struct {
	HookEvent string `json:"hook_event"`
	HookName  string `json:"hook_name"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome,omitempty"`
	Decision  string `json:"decision,omitempty"`
	Error     string `json:"error,omitempty"`
}

func globalHookDispatchSummary(
	hook hookspkg.RegisteredHook,
	phase hookspkg.DispatchPhase,
	outcome hookspkg.HookRunOutcome,
) string {
	if phase == hookspkg.DispatchPhaseComplete && strings.TrimSpace(string(outcome)) != "" {
		return fmt.Sprintf(
			"%s %s %s",
			hook.Event.String(),
			strings.TrimSpace(hook.Name),
			strings.TrimSpace(string(outcome)),
		)
	}
	return fmt.Sprintf("%s %s %s", hook.Event.String(), strings.TrimSpace(hook.Name), string(phase))
}

func hookDispatchErrorString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
