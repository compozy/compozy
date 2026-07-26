package daemon

import (
	"context"

	"fmt"

	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

func (n *hooksNotifier) dispatchAgentEvent(ctx context.Context, sessionCtx hookspkg.SessionContext, event any) {
	hooks, agentEventNotify := n.runtime()
	if agentEventNotify != nil {
		agentEventNotify.OnAgentEvent(ctx, sessionCtx.SessionID, event)
	}
	if hooks != nil {
		dispatchACPAgentHookEvent(ctx, n.logger, hooks, sessionCtx, event, n.timestamp())
	}
}

func (n *hooksNotifier) runtime() (hookRuntime, session.Notifier) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.hooks, n.agentEventNotify
}

func (n *hooksNotifier) hookEventSummaries() hookEventSummaryWriter {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.eventSummaries
}

func (n *hooksNotifier) timestamp() time.Time {
	if n == nil || n.now == nil {
		return time.Now().UTC()
	}
	return n.now().UTC()
}

type runtimeDispatchFunc[P any] func(hookRuntime, context.Context, P) (P, error)

func dispatchRuntime[P any](
	ctx context.Context,
	n *hooksNotifier,
	event hookspkg.HookEvent,
	payload P,
	dispatch runtimeDispatchFunc[P],
) (P, error) {
	hooks, _ := n.runtime()
	if hooks == nil {
		return payload, nil
	}
	if ctx == nil {
		return payload, fmt.Errorf("daemon: dispatch %s requires a non-nil context", event)
	}
	ctx = withGlobalHookDispatchEventEmitter(ctx, n, payload)
	return dispatch(hooks, ctx, payload)
}

func hookSessionLifecyclePayload(
	sess *session.Session,
	event hookspkg.HookEvent,
	timestamp time.Time,
) hookspkg.SessionLifecyclePayload {
	return hookspkg.SessionLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     event,
			Timestamp: timestamp,
		},
		SessionContext: hookSessionContext(sess),
	}
}

func hookSessionContext(sess *session.Session) hookspkg.SessionContext {
	if sess == nil {
		return hookspkg.SessionContext{}
	}

	info := sess.Info()
	if info == nil {
		return hookspkg.SessionContext{}
	}

	return hookspkg.SessionContext{
		SessionID:    strings.TrimSpace(info.ID),
		SessionName:  strings.TrimSpace(info.Name),
		SessionType:  string(info.Type),
		AgentName:    strings.TrimSpace(info.AgentName),
		WorkspaceID:  strings.TrimSpace(info.WorkspaceID),
		Workspace:    strings.TrimSpace(info.Workspace),
		ACPSessionID: strings.TrimSpace(info.ACPSessionID),
		State:        string(info.State),
		SessionSoulContext: hookSessionSoulContext(
			info.SoulSnapshotID,
			info.SoulDigest,
		),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func hookSessionSoulContext(snapshotID string, digest string) *hookspkg.SessionSoulContext {
	trimmedSnapshotID := strings.TrimSpace(snapshotID)
	trimmedDigest := strings.TrimSpace(digest)
	if trimmedSnapshotID == "" && trimmedDigest == "" {
		return nil
	}
	return &hookspkg.SessionSoulContext{
		SoulSnapshotID: trimmedSnapshotID,
		SoulDigest:     trimmedDigest,
	}
}

func sessionFromHookPayload(payload hookspkg.SessionLifecyclePayload) *session.Session {
	return &session.Session{
		ID:           strings.TrimSpace(payload.SessionID),
		Name:         strings.TrimSpace(payload.SessionName),
		AgentName:    strings.TrimSpace(payload.AgentName),
		WorkspaceID:  strings.TrimSpace(payload.WorkspaceID),
		Workspace:    strings.TrimSpace(payload.Workspace),
		Type:         session.Type(strings.TrimSpace(payload.SessionType)),
		State:        session.State(strings.TrimSpace(payload.State)),
		ACPSessionID: strings.TrimSpace(payload.ACPSessionID),
		CreatedAt:    payload.CreatedAt,
		UpdatedAt:    payload.UpdatedAt,
	}
}

func observeSessionCreateExecutor(observer sessionLifecycleObserver) hookspkg.Executor {
	return hookspkg.NewTypedNativeExecutor(
		func(
			ctx context.Context,
			_ hookspkg.RegisteredHook,
			payload hookspkg.SessionLifecyclePayload,
		) (hookspkg.SessionPostCreatePatch, error) {
			observer.OnSessionCreated(ctx, sessionFromHookPayload(payload))
			return hookspkg.SessionPostCreatePatch{}, nil
		},
	)
}

func observeSessionStopExecutor(observer sessionLifecycleObserver) hookspkg.Executor {
	return hookspkg.NewTypedNativeExecutor(
		func(
			ctx context.Context,
			_ hookspkg.RegisteredHook,
			payload hookspkg.SessionLifecyclePayload,
		) (hookspkg.SessionPostStopPatch, error) {
			observer.OnSessionStopped(ctx, sessionFromHookPayload(payload))
			return hookspkg.SessionPostStopPatch{}, nil
		},
	)
}

func dreamSessionStopExecutor(dreamRuntime dreamCheckEnqueuer) hookspkg.Executor {
	return hookspkg.NewTypedNativeExecutor(
		func(
			_ context.Context,
			_ hookspkg.RegisteredHook,
			payload hookspkg.SessionLifecyclePayload,
		) (hookspkg.SessionPostStopPatch, error) {
			if strings.TrimSpace(payload.WorkspaceID) == "" ||
				session.Type(strings.TrimSpace(payload.SessionType)) == session.SessionTypeDream {
				return hookspkg.SessionPostStopPatch{}, nil
			}

			dreamRuntime.EnqueueCheck("session_stop", strings.TrimSpace(payload.WorkspaceID))
			return hookspkg.SessionPostStopPatch{}, nil
		},
	)
}

func sessionMessagePersistedExecutor(
	observer sessionMessagePersistedObserver,
) hookspkg.Executor {
	return hookspkg.NewTypedNativeExecutor(
		func(
			ctx context.Context,
			_ hookspkg.RegisteredHook,
			payload hookspkg.SessionMessagePersistedPayload,
		) (hookspkg.AuthoredContextObservationPatch, error) {
			if err := observer.HandleSessionMessagePersisted(ctx, payload); err != nil {
				return hookspkg.AuthoredContextObservationPatch{}, err
			}
			return hookspkg.AuthoredContextObservationPatch{}, nil
		},
	)
}
