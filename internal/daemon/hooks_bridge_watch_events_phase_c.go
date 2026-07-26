package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

type coordinatorWatchObserver interface {
	OnCoordinatorSpawned(context.Context, hookspkg.CoordinatorSpawnedPayload) error
	OnCoordinatorDecision(context.Context, hookspkg.CoordinatorDecisionPayload) error
	OnCoordinatorStopped(context.Context, hookspkg.CoordinatorStoppedPayload) error
	OnCoordinatorFailed(context.Context, hookspkg.CoordinatorFailedPayload) error
}

type eventRecordWatchObserver interface {
	OnEventPostRecord(context.Context, hookspkg.EventPostRecordPayload) error
}

func (n *hooksNotifier) AddCoordinatorWatchObserver(observer coordinatorWatchObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.coordinatorWatchHooks = append(n.coordinatorWatchHooks, observer)
}

func (n *hooksNotifier) AddEventRecordWatchObserver(observer eventRecordWatchObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.eventRecordWatchHooks = append(n.eventRecordWatchHooks, observer)
}

func (n *hooksNotifier) coordinatorWatchObservers() []coordinatorWatchObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]coordinatorWatchObserver(nil), n.coordinatorWatchHooks...)
}

func (n *hooksNotifier) eventRecordWatchObservers() []eventRecordWatchObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]eventRecordWatchObserver(nil), n.eventRecordWatchHooks...)
}

func dispatchEventPostRecordWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookEventPostRecord,
		payload,
		hookRuntime.DispatchEventPostRecord,
	)
	notifier.notifyEventPostRecordObservers(ctx, result)
	return result, err
}

func dispatchCoordinatorSpawnedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.CoordinatorSpawnedPayload,
) (hookspkg.CoordinatorSpawnedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookCoordinatorSpawned,
		payload,
		hookRuntime.DispatchCoordinatorSpawned,
	)
	if writeErr := notifier.writeCoordinatorWatchEvent(ctx, hookspkg.HookCoordinatorSpawned, result); writeErr != nil {
		err = joinHookDispatchErrors(err, writeErr)
	}
	notifier.notifyCoordinatorWatchObservers(
		ctx,
		hookspkg.HookCoordinatorSpawned,
		result,
		func(ctx context.Context, observer coordinatorWatchObserver, payload hookspkg.CoordinatorLifecyclePayload) error {
			return observer.OnCoordinatorSpawned(ctx, payload)
		},
	)
	return result, err
}

func dispatchCoordinatorDecisionWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.CoordinatorDecisionPayload,
) (hookspkg.CoordinatorDecisionPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookCoordinatorDecision,
		payload,
		hookRuntime.DispatchCoordinatorDecision,
	)
	if writeErr := notifier.writeCoordinatorWatchEvent(ctx, hookspkg.HookCoordinatorDecision, result); writeErr != nil {
		err = joinHookDispatchErrors(err, writeErr)
	}
	notifier.notifyCoordinatorWatchObservers(
		ctx,
		hookspkg.HookCoordinatorDecision,
		result,
		func(ctx context.Context, observer coordinatorWatchObserver, payload hookspkg.CoordinatorLifecyclePayload) error {
			return observer.OnCoordinatorDecision(ctx, payload)
		},
	)
	return result, err
}

func dispatchCoordinatorStoppedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.CoordinatorStoppedPayload,
) (hookspkg.CoordinatorStoppedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookCoordinatorStopped,
		payload,
		hookRuntime.DispatchCoordinatorStopped,
	)
	if writeErr := notifier.writeCoordinatorWatchEvent(ctx, hookspkg.HookCoordinatorStopped, result); writeErr != nil {
		err = joinHookDispatchErrors(err, writeErr)
	}
	notifier.notifyCoordinatorWatchObservers(
		ctx,
		hookspkg.HookCoordinatorStopped,
		result,
		func(ctx context.Context, observer coordinatorWatchObserver, payload hookspkg.CoordinatorLifecyclePayload) error {
			return observer.OnCoordinatorStopped(ctx, payload)
		},
	)
	return result, err
}

func dispatchCoordinatorFailedWithWatchObservers(
	ctx context.Context,
	notifier *hooksNotifier,
	payload hookspkg.CoordinatorFailedPayload,
) (hookspkg.CoordinatorFailedPayload, error) {
	result, err := dispatchRuntime(
		ctx,
		notifier,
		hookspkg.HookCoordinatorFailed,
		payload,
		hookRuntime.DispatchCoordinatorFailed,
	)
	if writeErr := notifier.writeCoordinatorWatchEvent(ctx, hookspkg.HookCoordinatorFailed, result); writeErr != nil {
		err = joinHookDispatchErrors(err, writeErr)
	}
	notifier.notifyCoordinatorWatchObservers(
		ctx,
		hookspkg.HookCoordinatorFailed,
		result,
		func(ctx context.Context, observer coordinatorWatchObserver, payload hookspkg.CoordinatorLifecyclePayload) error {
			return observer.OnCoordinatorFailed(ctx, payload)
		},
	)
	return result, err
}

func (n *hooksNotifier) notifyEventPostRecordObservers(
	ctx context.Context,
	payload hookspkg.EventPostRecordPayload,
) {
	for _, observer := range n.eventRecordWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"event record",
			[]any{"session_id", payload.SessionID, "sequence", payload.Sequence},
			func(ctx context.Context, observer eventRecordWatchObserver, payload hookspkg.EventPostRecordPayload) error {
				return observer.OnEventPostRecord(ctx, payload)
			},
		)
	}
}

func (n *hooksNotifier) notifyCoordinatorWatchObservers(
	ctx context.Context,
	event hookspkg.HookEvent,
	payload hookspkg.CoordinatorLifecyclePayload,
	call func(context.Context, coordinatorWatchObserver, hookspkg.CoordinatorLifecyclePayload) error,
) {
	for _, observer := range n.coordinatorWatchObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"coordinator",
			[]any{
				daemonHookEventKey, event,
				"coordinator_session_id", payload.CoordinatorSessionID,
				daemonWorkspaceIDKey, payload.WorkspaceID,
			},
			call,
		)
	}
}

func (n *hooksNotifier) writeCoordinatorWatchEvent(
	ctx context.Context,
	event hookspkg.HookEvent,
	payload hookspkg.CoordinatorLifecyclePayload,
) error {
	writer := n.hookEventSummaries()
	if writer == nil {
		return nil
	}
	content, err := json.Marshal(map[string]any{
		watchEventsPayloadAgentNameKey:                    strings.TrimSpace(payload.AgentName),
		watchEventsPayloadCoordinatorSessionIDKey:         strings.TrimSpace(payload.CoordinatorSessionID),
		watchEventsPayloadResolvedNetworkParticipationKey: payload.ResolvedNetworkParticipation,
		watchEventsPayloadWorkflowIDKey:                   strings.TrimSpace(payload.WorkflowID),
		watchEventsPayloadProviderKey:                     strings.TrimSpace(payload.Provider),
		watchEventsPayloadModelKey:                        strings.TrimSpace(payload.Model),
		watchEventsPayloadDecisionKindKey:                 strings.TrimSpace(payload.DecisionKind),
		watchEventsPayloadDecisionKey:                     strings.TrimSpace(payload.Decision),
		watchEventsPayloadStopReasonKey:                   strings.TrimSpace(payload.StopReason),
		watchEventsPayloadErrorKey:                        strings.TrimSpace(payload.Error),
	})
	if err != nil {
		return fmt.Errorf("daemon: marshal coordinator watch event: %w", err)
	}
	return writer.WriteEventSummary(ctx, store.EventSummary{
		SessionID:   strings.TrimSpace(payload.CoordinatorSessionID),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		Type:        string(event),
		AgentName:   strings.TrimSpace(payload.AgentName),
		Provider:    strings.TrimSpace(payload.Provider),
		Outcome:     coordinatorWatchEventOutcome(event),
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			TaskID:               strings.TrimSpace(payload.TaskID),
			RunID:                strings.TrimSpace(payload.RunID),
			WorkflowID:           strings.TrimSpace(payload.WorkflowID),
			CoordinatorSessionID: strings.TrimSpace(payload.CoordinatorSessionID),
			HookEvent:            string(event),
		},
		Summary:   coordinatorWatchEventSummary(event, payload),
		Timestamp: payload.Timestamp.UTC(),
	})
}

func coordinatorWatchEventOutcome(event hookspkg.HookEvent) string {
	if event == hookspkg.HookCoordinatorFailed {
		return string(eventspkg.OutcomeFailure)
	}
	return string(eventspkg.OutcomeInfo)
}

func coordinatorWatchEventSummary(
	event hookspkg.HookEvent,
	payload hookspkg.CoordinatorLifecyclePayload,
) string {
	switch event {
	case hookspkg.HookCoordinatorFailed:
		return "coordinator failed " + strings.TrimSpace(payload.Error)
	case hookspkg.HookCoordinatorStopped:
		return "coordinator stopped " + strings.TrimSpace(payload.StopReason)
	default:
		return strings.TrimSpace(string(event)) + " " + strings.TrimSpace(payload.Decision)
	}
}

func joinHookDispatchErrors(existing error, next error) error {
	return errors.Join(existing, next)
}
