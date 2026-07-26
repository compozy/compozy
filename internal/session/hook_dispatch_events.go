package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

type hookDispatchEventEmitter struct {
	manager *Manager
	session *Session
}

var _ hookspkg.DispatchEventEmitter = hookDispatchEventEmitter{}

func (e hookDispatchEventEmitter) EmitHookDispatchEvent(
	ctx context.Context,
	payload any,
	hook hookspkg.RegisteredHook,
	phase hookspkg.DispatchPhase,
	outcome hookspkg.HookRunOutcome,
	err error,
	_ int,
	timestamp time.Time,
) {
	if e.manager == nil || e.session == nil {
		return
	}
	recorder := e.session.recorderHandle()
	if recorder == nil {
		return
	}

	info := e.session.Info()
	if info == nil {
		return
	}

	turnID := hookspkg.TurnIDFromPayload(payload)
	if turnID == "" {
		turnID = e.session.CurrentTurnID()
	}
	if turnID == "" {
		turnID = newID("turn")
	}
	if timestamp.IsZero() {
		timestamp = e.manager.now()
	}

	dispatchCorrelation := hookspkg.CorrelationFromPayload(payload)
	correlation := store.EventCorrelation{
		TaskID:               dispatchCorrelation.TaskID,
		RunID:                dispatchCorrelation.RunID,
		WorkflowID:           dispatchCorrelation.WorkflowID,
		CoordinatorSessionID: dispatchCorrelation.CoordinatorSessionID,
		HookEvent:            hook.Event.String(),
		HookName:             strings.TrimSpace(hook.Name),
		ActorKind:            dispatchCorrelation.ActorKind,
		ActorID:              dispatchCorrelation.ActorID,
		ReleaseReason:        dispatchCorrelation.ReleaseReason,
	}
	if correlation.ActorKind == "" {
		correlation.ActorKind = "agent_session"
	}
	if correlation.ActorID == "" {
		correlation.ActorID = strings.TrimSpace(info.ID)
	}

	event := e.manager.enrichRecordedAgentEvent(e.session, acp.AgentEvent{
		Type:             "hook.dispatch." + string(phase),
		SessionID:        strings.TrimSpace(info.ACPSessionID),
		TurnID:           turnID,
		EventCorrelation: correlation,
		Timestamp:        timestamp,
		Text:             hookDispatchSummary(hook, phase, outcome),
		Decision:         hookDispatchDecision(phase, outcome),
		Error:            hookDispatchError(err),
	})

	marshaled, marshalErr := marshalAgentEvent(event)
	if marshalErr != nil {
		e.manager.sessionLogger(e.session).Warn("session: marshal hook dispatch event failed", "error", marshalErr)
		return
	}
	if recordErr := recorder.Record(ctx, store.SessionEvent{
		TurnID:    turnID,
		Type:      event.Type,
		AgentName: info.AgentName,
		Content:   marshaled,
		Timestamp: timestamp,
	}); recordErr != nil {
		e.manager.sessionLogger(e.session).Warn("session: record hook dispatch event failed", "error", recordErr)
		return
	}

	e.manager.notifyAgentEvent(ctx, e.session, event)
}

func hookDispatchSummary(
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

func hookDispatchDecision(phase hookspkg.DispatchPhase, outcome hookspkg.HookRunOutcome) string {
	if phase != hookspkg.DispatchPhaseComplete {
		return ""
	}
	return strings.TrimSpace(string(outcome))
}

func hookDispatchError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
