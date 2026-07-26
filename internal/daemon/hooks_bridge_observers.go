package daemon

import (
	"context"
	"errors"

	"sync"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

type sessionLifecycleObserver interface {
	OnSessionCreated(context.Context, *session.Session)
	OnSessionStopped(context.Context, *session.Session)
}

type sessionFinalizationObserver interface {
	OnSessionFinalizing(context.Context, *session.Session)
}

type taskRunEnqueuedObserver interface {
	OnTaskRunEnqueued(context.Context, hookspkg.TaskRunEnqueuedPayload)
}

type taskRunTerminalObserver interface {
	OnTaskRunTerminal(context.Context, hookspkg.TaskRunLeasePayload) error
}

type dreamCheckEnqueuer interface {
	EnqueueCheck(reason string, workspaceRef string)
}

type sessionMessagePersistedObserver interface {
	HandleSessionMessagePersisted(context.Context, hookspkg.SessionMessagePersistedPayload) error
}

type sessionLifecycleFanout struct {
	mu        sync.RWMutex
	observers []sessionLifecycleObserver
}

func newSessionLifecycleFanout(observers ...sessionLifecycleObserver) *sessionLifecycleFanout {
	fanout := &sessionLifecycleFanout{}
	for _, observer := range observers {
		fanout.Add(observer)
	}
	return fanout
}

func (f *sessionLifecycleFanout) Add(observer sessionLifecycleObserver) {
	if f == nil || observer == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observers = append(f.observers, observer)
}

func (f *sessionLifecycleFanout) OnSessionCreated(ctx context.Context, sess *session.Session) {
	for _, observer := range f.snapshot() {
		observer.OnSessionCreated(ctx, sess)
	}
}

func (f *sessionLifecycleFanout) OnSessionStopped(ctx context.Context, sess *session.Session) {
	for _, observer := range f.snapshot() {
		observer.OnSessionStopped(ctx, sess)
	}
}

func (f *sessionLifecycleFanout) OnSessionFinalizing(ctx context.Context, sess *session.Session) {
	for _, observer := range f.snapshot() {
		if finalizer, ok := observer.(sessionFinalizationObserver); ok {
			finalizer.OnSessionFinalizing(ctx, sess)
		}
	}
}

func (f *sessionLifecycleFanout) OnSubprocessHealth(
	ctx context.Context,
	observation session.SubprocessHealthSnapshot,
) {
	for _, observer := range f.snapshot() {
		if notifier, ok := observer.(session.SubprocessHealthNotifier); ok {
			notifier.OnSubprocessHealth(ctx, observation)
		}
	}
}

func (f *sessionLifecycleFanout) snapshot() []sessionLifecycleObserver {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]sessionLifecycleObserver(nil), f.observers...)
}

type hookTelemetryFanout struct {
	mu    sync.RWMutex
	sinks []hookspkg.TelemetrySink
}

func newHookTelemetryFanout(sinks ...hookspkg.TelemetrySink) *hookTelemetryFanout {
	fanout := &hookTelemetryFanout{}
	for _, sink := range sinks {
		fanout.Add(sink)
	}
	return fanout
}

func (f *hookTelemetryFanout) Add(sink hookspkg.TelemetrySink) {
	if f == nil || sink == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sinks = append(f.sinks, sink)
}

func (f *hookTelemetryFanout) WriteHookRecord(
	ctx context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) error {
	var errs []error
	for _, sink := range f.snapshot() {
		if err := sink.WriteHookRecord(ctx, sessionID, record); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f *hookTelemetryFanout) snapshot() []hookspkg.TelemetrySink {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]hookspkg.TelemetrySink(nil), f.sinks...)
}

func (n *hooksNotifier) notifyTaskRunTerminalObservers(
	ctx context.Context,
	payload hookspkg.TaskRunLeasePayload,
) {
	for _, observer := range n.taskRunTerminalObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"task-run terminal",
			[]any{
				daemonTaskIDKey, payload.TaskID,
				daemonLogRunIDKey, payload.RunID,
				daemonLoopRunIDKey, payload.LoopRunID,
			},
			func(ctx context.Context, observer taskRunTerminalObserver, payload hookspkg.TaskRunLeasePayload) error {
				return observer.OnTaskRunTerminal(ctx, payload)
			},
		)
	}
}
